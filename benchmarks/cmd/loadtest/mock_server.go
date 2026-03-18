package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

type mockServer struct {
	tokenCount int
	chunkDelay time.Duration
	mode       string
}

type embeddingResponse struct {
	Object string              `json:"object"`
	Data   []embeddingData     `json:"data"`
	Model  string              `json:"model"`
	Usage  embeddingUsage      `json:"usage"`
}

type embeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type embeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type chatRequest struct {
	Stream bool `json:"stream"`
}

func (m *mockServer) pickMode() string {
	if m.mode != "mixed" {
		return m.mode
	}
	if rand.Float64() < 0.8 {
		return "confident"
	}
	return "uncertain"
}

func confidentLogprobs() string {
	return fmt.Sprintf(
		`"logprobs":{"content":[{"token":"tok","logprob":%f,"top_logprobs":[{"token":"tok","logprob":%f},{"token":"alt1","logprob":%f},{"token":"alt2","logprob":%f}]}]}`,
		math.Log(0.95), math.Log(0.95), math.Log(0.03), math.Log(0.02),
	)
}

func uncertainLogprobs() string {
	return fmt.Sprintf(
		`"logprobs":{"content":[{"token":"tok","logprob":%f,"top_logprobs":[{"token":"tok","logprob":%f},{"token":"alt1","logprob":%f},{"token":"alt2","logprob":%f}]}]}`,
		math.Log(0.34), math.Log(0.34), math.Log(0.33), math.Log(0.33),
	)
}

func (m *mockServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mode := m.pickMode()

	if !req.Stream {
		m.handleComplete(w, mode)
		return
	}
	m.handleStream(w, mode)
}

func (m *mockServer) handleComplete(w http.ResponseWriter, mode string) {
	logprobs := confidentLogprobs()
	if mode == "uncertain" {
		logprobs = uncertainLogprobs()
	}

	var b strings.Builder
	for i := range m.tokenCount {
		fmt.Fprintf(&b, "word%d ", i)
	}
	content := b.String()

	resp := fmt.Sprintf(
		`{"id":"mock-complete","object":"chat.completion","created":%d,"model":"gpt-4.1-nano","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},%s,"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":%d,"total_tokens":%d}}`,
		time.Now().Unix(), content, logprobs, m.tokenCount, m.tokenCount+10,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}

func (m *mockServer) handleStream(w http.ResponseWriter, mode string) {
	logprobs := confidentLogprobs()
	if mode == "uncertain" {
		logprobs = uncertainLogprobs()
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for i := range m.tokenCount {
		tok := fmt.Sprintf("word%d ", i)
		chunk := fmt.Sprintf(
			`data: {"id":"mock-%d","object":"chat.completion.chunk","model":"gpt-4.1-nano","choices":[{"index":0,"delta":{"content":"%s"},%s}]}`,
			i, tok, logprobs,
		)
		fmt.Fprintln(w, chunk)
		fmt.Fprintln(w)
		flusher.Flush()

		if m.chunkDelay > 0 {
			time.Sleep(m.chunkDelay)
		}
	}

	fmt.Fprintln(w, "data: [DONE]")
	fmt.Fprintln(w)
	flusher.Flush()
}

func (m *mockServer) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vec := make([]float64, 1536)
	for i := range vec {
		vec[i] = rand.Float64()*2 - 1
	}

	resp := embeddingResponse{
		Object: "list",
		Data: []embeddingData{
			{
				Object:    "embedding",
				Embedding: vec,
				Index:     0,
			},
		},
		Model: "text-embedding-3-small",
		Usage: embeddingUsage{
			PromptTokens: 10,
			TotalTokens:  10,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func startMockServer(addr string, mode string, tokenCount int, chunkDelay time.Duration) *http.Server {
	m := &mockServer{
		tokenCount: tokenCount,
		chunkDelay: chunkDelay,
		mode:       mode,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", m.handleChat)
	mux.HandleFunc("/v1/embeddings", m.handleEmbeddings)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(fmt.Sprintf("mock server listen failed: %v", err))
	}

	go srv.Serve(ln)

	return srv
}
