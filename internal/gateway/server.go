package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/trnahnh/draft-thinker/internal/config"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/internal/router"
	"github.com/trnahnh/draft-thinker/pkg/client"
)

type Server struct {
	httpServer *http.Server
	cfg        *config.Config
}

func NewServer(cfg *config.Config, drafter, heavyweight client.LLMClient, rtr *router.Router, rec metrics.Recorder) *Server {
	mux := http.NewServeMux()

	chatH := newChatHandler(drafter, heavyweight, rtr, rec, cfg.Entropy)
	mux.Handle("/v1/chat/completions", chatH)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	if cfg.Metrics.Enabled {
		if promRec, ok := rec.(*metrics.PrometheusRecorder); ok {
			mux.Handle(cfg.Metrics.Path, promRec.Handler())
		}
	}

	handler := chain(mux,
		recoveryMiddleware,
		loggingMiddleware,
		requestIDMiddleware,
	)

	return &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:      handler,
			ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
		},
	}
}

func (s *Server) Start() error {
	log.Printf("starting gateway on %s", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("shutting down gateway")
	return s.httpServer.Shutdown(ctx)
}
