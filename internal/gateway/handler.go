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

// Routing decision labels exposed on X-Routing-Decision and the
// routing_decisions_total metric.
const (
	decisionAccept              = "accept"
	decisionEscalate            = "escalate"
	decisionCacheHit            = "cache_hit"
	decisionFallbackHeavyweight = "fallback_heavyweight"
	decisionFallbackDraft       = "fallback_draft"
)

type chatHandler struct {
	drafter     client.LLMClient
	heavyweight client.LLMClient
	router      *router.Router
	executor    *speculative.Executor
	recorder    metrics.Recorder
	entropyCfg  config.EntropyConfig
	specCfg     config.SpeculativeConfig
	fallbackCfg config.FallbackConfig
	cache       cache.Store
}

func newChatHandler(drafter, heavyweight client.LLMClient, rtr *router.Router, exec *speculative.Executor, rec metrics.Recorder, entropyCfg config.EntropyConfig, specCfg config.SpeculativeConfig, fallbackCfg config.FallbackConfig, cacheStore cache.Store) *chatHandler {
	return &chatHandler{
		drafter:     drafter,
		heavyweight: heavyweight,
		router:      rtr,
		executor:    exec,
		recorder:    rec,
		entropyCfg:  entropyCfg,
		specCfg:     specCfg,
		fallbackCfg: fallbackCfg,
		cache:       cacheStore,
	}
}

func (h *chatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

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
			h.recorder.RecordRoutingDecision(decisionCacheHit)
			h.recorder.RecordRequest(resp.Model, http.StatusOK)
			if req.Stream {
				flusher, ok := w.(http.Flusher)
				if !ok {
					writeInternalError(w, "streaming not supported")
					return
				}
				setRoutingHeaders(w, decisionCacheHit, start)
				h.replayCachedSSE(w, flusher, resp)
			} else {
				setRoutingHeaders(w, decisionCacheHit, start)
				writeJSON(w, resp)
			}
			return
		}
	}

	if req.Stream {
		h.handleStream(w, r, &req, start)
	} else {
		h.handleComplete(w, r, &req, start)
	}
}

func (h *chatHandler) handleComplete(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time) {
	if h.specCfg.IsEnabled() && h.executor != nil {
		h.handleCompleteSpeculative(w, r, req, start)
		return
	}
	h.handleCompleteSerial(w, r, req, start)
}

func (h *chatHandler) handleCompleteSerial(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time) {
	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		h.recorder.RecordUpstreamLatency("drafter", time.Since(start))
		h.drafterFailed(w, r, req, start, err)
		return
	}

	result, err := h.router.Route(r.Context(), ch)
	h.recorder.RecordUpstreamLatency("drafter", time.Since(start))

	if err != nil {
		h.drafterFailed(w, r, req, start, err)
		return
	}

	if result.Decision == entropy.Escalate {
		h.recorder.RecordRoutingDecision(decisionEscalate)
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		hvStart := time.Now()
		resp, err := h.heavyweight.Complete(r.Context(), req)
		h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))

		if err != nil {
			h.heavyweightFailed(w, r, req, start, err, false)
			return
		}

		h.recorder.RecordRequest(resp.Model, http.StatusOK)
		setRoutingHeaders(w, decisionEscalate, start)
		writeJSON(w, resp)
		return
	}

	h.recorder.RecordRoutingDecision(decisionAccept)
	resp := assembleResponse(result.DraftChunks)
	h.cacheInsert(req.Messages, resp)
	h.recorder.RecordRequest(resp.Model, http.StatusOK)
	setRoutingHeaders(w, decisionAccept, start)
	writeJSON(w, resp)
}

func (h *chatHandler) handleCompleteSpeculative(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time) {
	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		h.recorder.RecordUpstreamLatency("drafter", time.Since(start))
		h.drafterFailed(w, r, req, start, err)
		return
	}

	result, err := h.executor.Execute(r.Context(), ch, req)
	h.recorder.RecordUpstreamLatency("drafter", time.Since(start))

	if err != nil {
		h.drafterFailed(w, r, req, start, err)
		return
	}

	switch result.State {
	case speculative.Escalated:
		h.recorder.RecordRoutingDecision(decisionEscalate)
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		if result.HeavyCh != nil {
			defer result.HeavyCancel()
			var hvChunks []protocol.StreamChunk
			for sc := range result.HeavyCh {
				if sc.Err != nil {
					h.heavyweightFailed(w, r, req, start, sc.Err, false)
					return
				}
				if sc.Chunk != nil {
					hvChunks = append(hvChunks, *sc.Chunk)
				}
			}
			resp := assembleResponse(hvChunks)
			h.recorder.RecordRequest(resp.Model, http.StatusOK)
			setRoutingHeaders(w, decisionEscalate, start)
			writeJSON(w, resp)
			return
		}

		hvStart := time.Now()
		resp, err := h.heavyweight.Complete(r.Context(), req)
		h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
		if err != nil {
			h.heavyweightFailed(w, r, req, start, err, false)
			return
		}
		h.recorder.RecordRequest(resp.Model, http.StatusOK)
		setRoutingHeaders(w, decisionEscalate, start)
		writeJSON(w, resp)

	default:
		h.recorder.RecordRoutingDecision(decisionAccept)
		resp := assembleResponse(result.DraftChunks)
		h.cacheInsert(req.Messages, resp)
		h.recorder.RecordRequest(resp.Model, http.StatusOK)
		setRoutingHeaders(w, decisionAccept, start)
		writeJSON(w, resp)
	}
}

func (h *chatHandler) handleStream(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time) {
	if h.specCfg.IsEnabled() && h.executor != nil {
		h.handleStreamSpeculative(w, r, req, start)
		return
	}
	h.handleStreamSerial(w, r, req, start)
}

func (h *chatHandler) handleStreamSerial(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time) {
	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		h.recorder.RecordUpstreamLatency("drafter", time.Since(start))
		h.drafterFailed(w, r, req, start, err)
		return
	}

	result, err := h.router.Route(r.Context(), ch)
	h.recorder.RecordUpstreamLatency("drafter", time.Since(start))

	if err != nil {
		h.drafterFailed(w, r, req, start, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeInternalError(w, "streaming not supported")
		return
	}

	if result.Decision == entropy.Escalate {
		h.recorder.RecordRoutingDecision(decisionEscalate)
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		hvStart := time.Now()
		hvCh, err := h.heavyweight.Stream(r.Context(), req)
		if err != nil {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
			h.heavyweightFailed(w, r, req, start, err, true)
			return
		}

		h.streamHeavyweight(w, r, req, flusher, start, hvCh, func() {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
		})
		return
	}

	h.recorder.RecordRoutingDecision(decisionAccept)
	resp := assembleResponse(result.DraftChunks)
	h.cacheInsert(req.Messages, resp)
	setRoutingHeaders(w, decisionAccept, start)
	h.replayDraftSSE(w, flusher, result.DraftChunks)
}

func (h *chatHandler) handleStreamSpeculative(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time) {
	drafterReq := h.cloneForDrafter(req)

	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		h.recorder.RecordUpstreamLatency("drafter", time.Since(start))
		h.drafterFailed(w, r, req, start, err)
		return
	}

	result, err := h.executor.Execute(r.Context(), ch, req)
	h.recorder.RecordUpstreamLatency("drafter", time.Since(start))

	if err != nil {
		h.drafterFailed(w, r, req, start, err)
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
		h.recorder.RecordRoutingDecision(decisionEscalate)
		log.Printf("escalating to heavyweight (tokens=%d, avg_entropy=%.3f)",
			result.TokenCount, avgEntropy(result.Entropies))

		if result.HeavyCh != nil {
			defer result.HeavyCancel()
			h.streamHeavyweight(w, r, req, flusher, start, result.HeavyCh, nil)
			return
		}

		hvStart := time.Now()
		hvCh, err := h.heavyweight.Stream(r.Context(), req)
		if err != nil {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
			h.heavyweightFailed(w, r, req, start, err, true)
			return
		}
		h.streamHeavyweight(w, r, req, flusher, start, hvCh, func() {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
		})

	default:
		h.recorder.RecordRoutingDecision(decisionAccept)
		resp := assembleResponse(result.DraftChunks)
		h.cacheInsert(req.Messages, resp)
		setRoutingHeaders(w, decisionAccept, start)
		h.replayDraftSSE(w, flusher, result.DraftChunks)
	}
}

// drafterFailed handles a drafter call that errored or timed out before a
// routing decision could be made. With fallback enabled the request is
// sent straight to the heavyweight so the client still gets an answer;
// otherwise the upstream error is surfaced as before.
func (h *chatHandler) drafterFailed(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time, err error) {
	if !h.fallbackCfg.OnDrafterErrorEnabled() {
		h.handleUpstreamError(w, err)
		return
	}

	h.recorder.RecordError("drafter_fallback")
	h.recorder.RecordRoutingDecision(decisionFallbackHeavyweight)
	log.Printf("drafter failed, falling back to heavyweight: %v", err)

	hvStart := time.Now()
	if req.Stream {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeInternalError(w, "streaming not supported")
			return
		}
		hvCh, err := h.heavyweight.Stream(r.Context(), req)
		if err != nil {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
			h.handleUpstreamError(w, err)
			return
		}
		setRoutingHeaders(w, decisionFallbackHeavyweight, start)
		w.Header().Set("X-Fallback-Reason", "drafter_error")
		h.streamSSE(w, flusher, hvCh, func() {
			h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
		})
		return
	}

	resp, err := h.heavyweight.Complete(r.Context(), req)
	h.recorder.RecordUpstreamLatency("heavyweight", time.Since(hvStart))
	if err != nil {
		h.handleUpstreamError(w, err)
		return
	}
	h.recorder.RecordRequest(resp.Model, http.StatusOK)
	setRoutingHeaders(w, decisionFallbackHeavyweight, start)
	w.Header().Set("X-Fallback-Reason", "drafter_error")
	writeJSON(w, resp)
}

// heavyweightFailed handles a heavyweight call that errored after the
// router decided to escalate. The draft collected so far is usually
// partial (escalation aborts the drafter stream), so with fallback enabled
// the drafter is re-run to completion and that answer is served as a
// degraded, uncached response flagged via X-Routing-Decision. If the
// drafter also fails, the original heavyweight error is surfaced.
func (h *chatHandler) heavyweightFailed(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, start time.Time, hvErr error, stream bool) {
	if !h.fallbackCfg.ServeDraftOnHeavyErrorEnabled() {
		h.handleUpstreamError(w, hvErr)
		return
	}

	h.recorder.RecordError("heavyweight_fallback")
	log.Printf("heavyweight failed after escalation, re-running drafter: %v", hvErr)

	drafterReq := h.cloneForDrafter(req)
	ch, err := h.drafter.Stream(r.Context(), drafterReq)
	if err != nil {
		log.Printf("drafter fallback also failed: %v", err)
		h.handleUpstreamError(w, hvErr)
		return
	}
	var chunks []protocol.StreamChunk
	for sc := range ch {
		if sc.Err != nil {
			log.Printf("drafter fallback stream failed: %v", sc.Err)
			h.handleUpstreamError(w, hvErr)
			return
		}
		if sc.Chunk != nil {
			chunks = append(chunks, *sc.Chunk)
		}
	}
	if len(chunks) == 0 {
		h.handleUpstreamError(w, hvErr)
		return
	}

	h.recorder.RecordRoutingDecision(decisionFallbackDraft)
	resp := assembleResponse(chunks)
	setRoutingHeaders(w, decisionFallbackDraft, start)
	w.Header().Set("X-Fallback-Reason", "heavyweight_error")

	if flusher, ok := w.(http.Flusher); ok && stream {
		h.replayDraftSSE(w, flusher, chunks)
		return
	}
	h.recorder.RecordRequest(resp.Model, http.StatusOK)
	writeJSON(w, resp)
}

// streamHeavyweight waits for the first heavyweight chunk before committing
// to an SSE response. If the stream fails before producing anything the
// request can still fall back to the drafter; once bytes have been written
// the only option is to end the stream.
func (h *chatHandler) streamHeavyweight(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest, flusher http.Flusher, start time.Time, hvCh <-chan client.StreamChunk, onFirstChunk func()) {
	first, ok := <-hvCh
	if !ok {
		h.heavyweightFailed(w, r, req, start, fmt.Errorf("heavyweight stream closed without data"), true)
		return
	}
	if first.Err != nil {
		h.heavyweightFailed(w, r, req, start, first.Err, true)
		return
	}

	merged := make(chan client.StreamChunk, 1)
	merged <- first
	go func() {
		defer close(merged)
		for sc := range hvCh {
			merged <- sc
		}
	}()

	setRoutingHeaders(w, decisionEscalate, start)
	h.streamSSE(w, flusher, merged, onFirstChunk)
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func setRoutingHeaders(w http.ResponseWriter, decision string, start time.Time) {
	w.Header().Set("X-Routing-Decision", decision)
	w.Header().Set("X-Request-Duration-Ms", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
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
