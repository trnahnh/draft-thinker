package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/trnahnh/draft-thinker/internal/cache"
	"github.com/trnahnh/draft-thinker/internal/config"
	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/internal/router"
	"github.com/trnahnh/draft-thinker/internal/speculative"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type chatHandler struct {
	drafter     client.LLMClient
	heavyweight client.LLMClient
	router      *router.Router
	executor    *speculative.Executor
	recorder    metrics.Recorder
	entropyCfg  config.EntropyConfig
	specCfg     config.SpeculativeConfig
	cache       cache.Store
}

func newChatHandler(drafter, heavyweight client.LLMClient, rtr *router.Router, exec *speculative.Executor, rec metrics.Recorder, entropyCfg config.EntropyConfig, specCfg config.SpeculativeConfig, cacheStore cache.Store) *chatHandler {
	return &chatHandler{
		drafter:     drafter,
		heavyweight: heavyweight,
		router:      rtr,
		executor:    exec,
		recorder:    rec,
		entropyCfg:  entropyCfg,
		specCfg:     specCfg,
		cache:       cacheStore,
	}
}

func (h *chatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeBadRequest(w, "method not allowed, use POST")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req protocol.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.recorder.RecordError("invalid_request")
		writeBadRequest(w, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if len(req.Messages) == 0 {
		h.recorder.RecordError("invalid_request")
		writeBadRequest(w, "messages array is required and must not be empty")
		return
	}

	if h.cache != nil {
		resp, err := h.cache.Lookup(r.Context(), req.Messages)
		if err != nil {
			log.Printf("cache lookup error: %v", err)
		} else if resp != nil {
			h.recorder.RecordRoutingDecision("cache_hit")
			h.recorder.RecordRequest(resp.Model, http.StatusOK)
			if req.Stream {
				flusher, ok := w.(http.Flusher)
				if !ok {
					writeInternalError(w, "streaming not supported")
					return
				}
				h.replayCachedSSE(w, flusher, resp)
			} else {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}
			return
		}
	}

	if req.Stream {
		h.handleStream(w, r, &req)
	} else {
		h.handleComplete(w, r, &req)
	}
}

func (h *chatHandler) handleComplete(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	if h.specCfg.IsEnabled() && h.executor != nil {
		h.handleCompleteSpeculative(w, r, req)
		return
	}
	h.handleCompleteSerial(w, r, req)
}

func (h *chatHandler) handleCompleteSerial(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	start := time.Now()

	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		elapsed := time.Since(start)
		h.recorder.RecordUpstreamLatency("drafter", elapsed)
		h.handleUpstreamError(w, err)
		return
	}

	result, err := h.router.Route(r.Context(), ch)
	elapsed := time.Since(start)
	h.recorder.RecordUpstreamLatency("drafter", elapsed)

	if err != nil {
		h.recorder.RecordError("routing_error")
		writeInternalError(w, "routing error")
		log.Printf("routing error: %v", err)
		return
	}

	if result.Decision == entropy.Escalate {
		h.recorder.RecordRoutingDecision("escalate")
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		hvStart := time.Now()
		resp, err := h.heavyweight.Complete(r.Context(), req)
		h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))

		if err != nil {
			h.handleUpstreamError(w, err)
			return
		}

		h.recorder.RecordRequest(resp.Model, http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	h.recorder.RecordRoutingDecision("accept")
	resp := assembleResponse(result.DraftChunks)
	h.cacheInsert(req.Messages, resp)
	h.recorder.RecordRequest(resp.Model, http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *chatHandler) handleCompleteSpeculative(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	start := time.Now()

	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		h.recorder.RecordUpstreamLatency("drafter", time.Since(start))
		h.handleUpstreamError(w, err)
		return
	}

	result, err := h.executor.Execute(r.Context(), ch, req)
	h.recorder.RecordUpstreamLatency("drafter", time.Since(start))

	if err != nil {
		h.recorder.RecordError("routing_error")
		writeInternalError(w, "routing error")
		log.Printf("speculative routing error: %v", err)
		return
	}

	switch result.State {
	case speculative.Escalated:
		h.recorder.RecordRoutingDecision("escalate")
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		if result.HeavyCh != nil {
			defer result.HeavyCancel()
			var hvChunks []protocol.StreamChunk
			for sc := range result.HeavyCh {
				if sc.Err != nil {
					h.recorder.RecordError("stream_error")
					log.Printf("speculative heavyweight stream error: %v", sc.Err)
					writeInternalError(w, "heavyweight stream error")
					return
				}
				if sc.Chunk != nil {
					hvChunks = append(hvChunks, *sc.Chunk)
				}
			}
			resp := assembleResponse(hvChunks)
			h.recorder.RecordRequest(resp.Model, http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		hvStart := time.Now()
		resp, err := h.heavyweight.Complete(r.Context(), req)
		h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
		if err != nil {
			h.handleUpstreamError(w, err)
			return
		}
		h.recorder.RecordRequest(resp.Model, http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		h.recorder.RecordRoutingDecision("accept")
		resp := assembleResponse(result.DraftChunks)
		h.cacheInsert(req.Messages, resp)
		h.recorder.RecordRequest(resp.Model, http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func (h *chatHandler) handleStream(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	if h.specCfg.IsEnabled() && h.executor != nil {
		h.handleStreamSpeculative(w, r, req)
		return
	}
	h.handleStreamSerial(w, r, req)
}

func (h *chatHandler) handleStreamSerial(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	start := time.Now()

	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		elapsed := time.Since(start)
		h.recorder.RecordUpstreamLatency("drafter", elapsed)
		h.handleUpstreamError(w, err)
		return
	}

	result, err := h.router.Route(r.Context(), ch)
	elapsed := time.Since(start)
	h.recorder.RecordUpstreamLatency("drafter", elapsed)

	if err != nil {
		h.recorder.RecordError("routing_error")
		writeInternalError(w, "routing error")
		log.Printf("routing error: %v", err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeInternalError(w, "streaming not supported")
		return
	}

	if result.Decision == entropy.Escalate {
		h.recorder.RecordRoutingDecision("escalate")
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		hvStart := time.Now()
		hvCh, err := h.heavyweight.Stream(r.Context(), req)
		if err != nil {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
			h.handleUpstreamError(w, err)
			return
		}

		h.streamSSE(w, flusher, hvCh, func() {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
		})
		return
	}

	h.recorder.RecordRoutingDecision("accept")
	resp := assembleResponse(result.DraftChunks)
	h.cacheInsert(req.Messages, resp)
	h.replayDraftSSE(w, flusher, result.DraftChunks)
}

func (h *chatHandler) handleStreamSpeculative(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	start := time.Now()

	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		h.recorder.RecordUpstreamLatency("drafter", time.Since(start))
		h.handleUpstreamError(w, err)
		return
	}

	result, err := h.executor.Execute(r.Context(), ch, req)
	h.recorder.RecordUpstreamLatency("drafter", time.Since(start))

	if err != nil {
		h.recorder.RecordError("routing_error")
		writeInternalError(w, "routing error")
		log.Printf("speculative routing error: %v", err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		if result.HeavyCancel != nil {
			result.HeavyCancel()
		}
		writeInternalError(w, "streaming not supported")
		return
	}

	switch result.State {
	case speculative.Escalated:
		h.recorder.RecordRoutingDecision("escalate")
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		if result.HeavyCh != nil {
			defer result.HeavyCancel()
			h.streamSSE(w, flusher, result.HeavyCh, nil)
			return
		}

		hvStart := time.Now()
		hvCh, err := h.heavyweight.Stream(r.Context(), req)
		if err != nil {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
			h.handleUpstreamError(w, err)
			return
		}
		h.streamSSE(w, flusher, hvCh, func() {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
		})

	default:
		h.recorder.RecordRoutingDecision("accept")
		resp := assembleResponse(result.DraftChunks)
		h.cacheInsert(req.Messages, resp)
		h.replayDraftSSE(w, flusher, result.DraftChunks)
	}
}

func (h *chatHandler) streamSSE(w http.ResponseWriter, flusher http.Flusher, ch <-chan client.StreamChunk, onFirstChunk func()) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	firstChunk := true
	model := "unknown"

	for sc := range ch {
		if sc.Err != nil {
			log.Printf("heavyweight stream error: %v", sc.Err)
			h.recorder.RecordError("stream_error")
			break
		}

		if firstChunk && onFirstChunk != nil {
			onFirstChunk()
			firstChunk = false
		}

		if sc.Chunk != nil {
			model = sc.Chunk.Model
			data, err := json.Marshal(sc.Chunk)
			if err != nil {
				log.Printf("marshal chunk error: %v", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				break
			}
			flusher.Flush()
		}
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
	h.recorder.RecordRequest(model, http.StatusOK)
}

func (h *chatHandler) replayDraftSSE(w http.ResponseWriter, flusher http.Flusher, chunks []protocol.StreamChunk) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	model := "unknown"
	for _, chunk := range chunks {
		model = chunk.Model
		data, err := json.Marshal(chunk)
		if err != nil {
			log.Printf("marshal chunk error: %v", err)
			continue
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			break
		}
		flusher.Flush()
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
	h.recorder.RecordRequest(model, http.StatusOK)
}

func (h *chatHandler) cloneForDrafter(req *protocol.ChatCompletionRequest) *protocol.ChatCompletionRequest {
	clone := *req
	clone.Messages = make([]protocol.Message, len(req.Messages))
	copy(clone.Messages, req.Messages)
	clone.Stream = true
	logprobs := true
	clone.Logprobs = &logprobs
	topLogprobs := h.entropyCfg.TopLogprobs
	clone.TopLogprobs = &topLogprobs
	return &clone
}

func (h *chatHandler) handleUpstreamError(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case *client.UpstreamError:
		h.recorder.RecordError("upstream_error")
		h.recorder.RecordRequest("unknown", e.StatusCode)
		writeUpstreamError(w, e.StatusCode, e.Error())
	case *client.UpstreamTimeoutError:
		h.recorder.RecordError("upstream_timeout")
		h.recorder.RecordRequest("unknown", http.StatusGatewayTimeout)
		writeError(w, http.StatusGatewayTimeout, e.Error(), "timeout_error")
	default:
		h.recorder.RecordError("internal_error")
		h.recorder.RecordRequest("unknown", http.StatusInternalServerError)
		writeInternalError(w, "internal server error")
		log.Printf("unexpected upstream error: %v", err)
	}
}

func (h *chatHandler) cacheInsert(messages []protocol.Message, resp *protocol.ChatCompletionResponse) {
	if h.cache == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.cache.Insert(ctx, messages, resp); err != nil {
			log.Printf("cache insert error: %v", err)
		}
	}()
}

func (h *chatHandler) replayCachedSSE(w http.ResponseWriter, flusher http.Flusher, resp *protocol.ChatCompletionResponse) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for _, choice := range resp.Choices {
		chunk := protocol.StreamChunk{
			ID:     resp.ID,
			Object: "chat.completion.chunk",
			Model:  resp.Model,
			Choices: []protocol.Choice{
				{
					Index: choice.Index,
					Delta: &protocol.Delta{
						Role:    "assistant",
						Content: choice.Message.Content,
					},
				},
			},
		}
		data, err := json.Marshal(chunk)
		if err != nil {
			log.Printf("marshal cached chunk error: %v", err)
			continue
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			break
		}
		flusher.Flush()
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func avgEntropy(entropies []entropy.TokenEntropy) float64 {
	if len(entropies) == 0 {
		return 0
	}
	sum := 0.0
	for _, e := range entropies {
		sum += e.Entropy
	}
	return sum / float64(len(entropies))
}
