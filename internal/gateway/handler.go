package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type chatHandler struct {
	client   client.LLMClient
	recorder metrics.Recorder
}

func newChatHandler(c client.LLMClient, rec metrics.Recorder) *chatHandler {
	return &chatHandler{client: c, recorder: rec}
}

func (h *chatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeBadRequest(w, "method not allowed, use POST")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

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

	if req.Stream {
		h.handleStream(w, r, &req)
	} else {
		h.handleComplete(w, r, &req)
	}
}

func (h *chatHandler) handleComplete(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	start := time.Now()

	resp, err := h.client.Complete(r.Context(), req)
	elapsed := time.Since(start)
	h.recorder.RecordUpstreamLatency("groq", elapsed)

	if err != nil {
		h.handleUpstreamError(w, err)
		return
	}

	h.recorder.RecordRequest(resp.Model, http.StatusOK)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *chatHandler) handleStream(w http.ResponseWriter, r *http.Request, req *protocol.ChatCompletionRequest) {
	start := time.Now()

	ch, err := h.client.Stream(r.Context(), req)
	if err != nil {
		elapsed := time.Since(start)
		h.recorder.RecordUpstreamLatency("groq", elapsed)
		h.handleUpstreamError(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeInternalError(w, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	firstChunk := true
	model := "unknown"

	for sc := range ch {
		if sc.Err != nil {
			log.Printf("stream error: %v", sc.Err)
			h.recorder.RecordError("stream_error")
			break
		}

		if firstChunk {
			elapsed := time.Since(start)
			h.recorder.RecordUpstreamLatency("groq", elapsed)
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
