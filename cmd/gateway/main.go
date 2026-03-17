package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trnahnh/draft-thinker/internal/config"
	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/gateway"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/internal/router"
	"github.com/trnahnh/draft-thinker/pkg/client"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey == "" {
		log.Fatal("GROQ_API_KEY environment variable is required")
	}

	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	var rec metrics.Recorder
	if cfg.Metrics.Enabled {
		rec = metrics.NewPrometheusRecorder()
	} else {
		rec = &metrics.NoopRecorder{}
	}

	drafter := client.NewGroqClient(
		cfg.Drafter.BaseURL,
		groqKey,
		cfg.Drafter.Model,
		time.Duration(cfg.Drafter.Timeout)*time.Second,
	)

	heavyweight := client.NewOpenAIClient(
		cfg.Heavyweight.BaseURL,
		openaiKey,
		cfg.Heavyweight.Model,
		time.Duration(cfg.Heavyweight.Timeout)*time.Second,
	)

	rtr := router.NewRouter(entropy.WindowConfig{
		Size:           cfg.Entropy.WindowSize,
		Threshold:      cfg.Entropy.Threshold,
		EarlyExitCount: cfg.Entropy.EarlyExitCount,
	}, rec)

	srv := gateway.NewServer(cfg, drafter, heavyweight, rtr, rec)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal: %v", sig)
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}

	log.Println("gateway stopped")
}
