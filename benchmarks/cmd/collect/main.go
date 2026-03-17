package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/benchmarks/internal/dataset"
	"github.com/trnahnh/draft-thinker/pkg/client"
)

func main() {
	prompts := flag.String("prompts", "benchmarks/testdata/prompts.json", "path to prompts JSON file")
	output := flag.String("output", "benchmarks/results/collected.jsonl", "output JSONL path")
	concurrency := flag.Int("concurrency", 3, "max concurrent API calls")
	rateDelay := flag.Duration("rate-delay", 500*time.Millisecond, "delay between prompt dispatches")
	topLogprobs := flag.Int("top-logprobs", 5, "number of top logprobs to request")
	maxTokens := flag.Int("max-tokens", 1024, "max tokens per completion")
	flag.Parse()

	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	ds, err := dataset.Load(*prompts)
	if err != nil {
		log.Fatalf("loading dataset: %v", err)
	}
	log.Printf("loaded %d prompts", len(ds.Prompts))

	store, err := collect.NewStore(*output)
	if err != nil {
		log.Fatalf("opening store: %v", err)
	}
	log.Printf("already collected: %d", store.Count())

	drafter := client.NewOpenAICompatibleClient(
		"https://api.openai.com/v1",
		openaiKey,
		"gpt-4.1-nano",
		"openai",
		30*time.Second,
	)
	heavyweight := client.NewOpenAICompatibleClient(
		"https://api.openai.com/v1",
		openaiKey,
		"gpt-4.1",
		"openai",
		60*time.Second,
	)
	judge := collect.NewJudge(heavyweight)

	collector := collect.NewCollector(drafter, heavyweight, judge, collect.CollectorConfig{
		TopLogprobs: *topLogprobs,
		MaxTokens:   *maxTokens,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup
	completed := store.Count()
	total := len(ds.Prompts)

	for _, p := range ds.Prompts {
		if ctx.Err() != nil {
			break
		}

		if store.AlreadyCollected(p.ID) {
			continue
		}

		select {
		case <-ctx.Done():
		case sem <- struct{}{}:
		}
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(prompt dataset.Prompt) {
			defer wg.Done()
			defer func() { <-sem }()

			record, err := collector.Collect(ctx, prompt)
			if err != nil {
				log.Printf("[%d/%d] %s: fatal error: %v", completed, total, prompt.ID, err)
				return
			}

			if record.Error != "" && isRateLimit(record.Error) {
				for retry := 0; retry < 3; retry++ {
					wait := time.Duration(30*(retry+1)) * time.Second
					log.Printf("[%d/%d] %s: rate limited, retrying in %v", completed, total, prompt.ID, wait)
					select {
					case <-time.After(wait):
					case <-ctx.Done():
						return
					}
					record, err = collector.Collect(ctx, prompt)
					if err != nil {
						log.Printf("[%d/%d] %s: retry fatal: %v", completed, total, prompt.ID, err)
						return
					}
					if record.Error == "" || !isRateLimit(record.Error) {
						break
					}
				}
			}

			if err := store.Append(record); err != nil {
				log.Printf("[%d/%d] %s: store error: %v", completed, total, prompt.ID, err)
				return
			}

			completed++
			status := "error"
			if record.Error == "" {
				status = fmt.Sprintf("acceptable=%v, mean_entropy=%.2f", record.Judge.Acceptable, record.MeanEntropy)
			}
			log.Printf("[%d/%d] %s: %s, %.1fs",
				completed, total, prompt.ID, status,
				float64(record.DraftLatencyMS+record.HeavyLatencyMS)/1000)
		}(p)

		select {
		case <-time.After(*rateDelay):
		case <-ctx.Done():
		}
	}

	wg.Wait()
	log.Printf("collection complete: %d/%d", store.Count(), total)
}

func isRateLimit(errStr string) bool {
	return len(errStr) > 0 && (contains429(errStr))
}

func contains429(s string) bool {
	return http.StatusTooManyRequests == 429 && len(s) > 3 &&
		(s[0:3] == "429" || indexOf(s, "429") >= 0 || indexOf(s, "rate") >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
