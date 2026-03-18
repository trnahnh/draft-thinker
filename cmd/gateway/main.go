package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trnahnh/draft-thinker/internal/cache"
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

	drafter := client.NewOpenAICompatibleClient(
		cfg.Drafter.BaseURL,
		openaiKey,
		cfg.Drafter.Model,
		cfg.Drafter.Provider,
		time.Duration(cfg.Drafter.Timeout)*time.Second,
	)

	heavyweight := client.NewOpenAICompatibleClient(
		cfg.Heavyweight.BaseURL,
		openaiKey,
		cfg.Heavyweight.Model,
		cfg.Heavyweight.Provider,
		time.Duration(cfg.Heavyweight.Timeout)*time.Second,
	)

	rtr := router.NewRouter(entropy.WindowConfig{
		Size:           cfg.Entropy.WindowSize,
		Threshold:      cfg.Entropy.Threshold,
		EarlyExitCount: cfg.Entropy.EarlyExitCount,
	}, rec)

	var cacheStore cache.Store
	if cfg.Cache.IsEnabled() {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			redisURL = "localhost:6379"
		}
		qdrantURL := os.Getenv("QDRANT_URL")
		if qdrantURL == "" {
			qdrantURL = "http://localhost:6333"
		}

		cacheStore, err = cache.New(
			cfg.Drafter.BaseURL,
			openaiKey,
			cfg.Cache.EmbeddingModel,
			cfg.Cache.EmbeddingDimensions,
			qdrantURL,
			cfg.Cache.QdrantCollection,
			redisURL,
			cfg.Cache.TTLSeconds,
			cfg.Cache.SimilarityThreshold,
			rec,
		)
		if err != nil {
			log.Fatalf("initializing cache: %v", err)
		}
		log.Printf("semantic cache enabled (threshold=%.2f, ttl=%ds)", cfg.Cache.SimilarityThreshold, cfg.Cache.TTLSeconds)
	}

	srv := gateway.NewServer(cfg, drafter, heavyweight, rtr, rec, cacheStore)

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
