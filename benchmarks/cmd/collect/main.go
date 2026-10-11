package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
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
	drafterBaseURL := flag.String("drafter-base-url", "https://api.openai.com/v1", "OpenAI-compatible base URL for the drafter")
	drafterModel := flag.String("drafter-model", "gpt-4.1-nano", "drafter model name")
	drafterKeyEnv := flag.String("drafter-key-env", "OPENAI_API_KEY", "env var holding the drafter API key (empty value allowed for local servers)")
	heavyBaseURL := flag.String("heavy-base-url", "https://api.openai.com/v1", "OpenAI-compatible base URL for the heavyweight and judge")
	heavyModel := flag.String("heavy-model", "gpt-4.1", "heavyweight (and judge) model name")
	drafterOnly := flag.Bool("drafter-only", false, "collect only the drafter trace; skip heavyweight reference and judge")
	categories := flag.String("categories", "", "comma-separated categories to include (default all)")
	limit := flag.Int("limit", 0, "max prompts per category after filtering (0 = no limit)")
	flag.Parse()

	drafterKey := os.Getenv(*drafterKeyEnv)
	if drafterKey == "" {
		if strings.Contains(*drafterBaseURL, "api.openai.com") {
			log.Fatalf("%s is required for %s", *drafterKeyEnv, *drafterBaseURL)
		}
		drafterKey = "none"
	}
	heavyKey := os.Getenv("OPENAI_API_KEY")
	if !*drafterOnly && heavyKey == "" {
		log.Fatal("OPENAI_API_KEY is required unless -drafter-only is set")
	}

	ds, err := dataset.Load(*prompts)
	if err != nil {
		log.Fatalf("loading dataset: %v", err)
	}
	selected := filterPrompts(ds.Prompts, *categories, *limit)
	log.Printf("loaded %d prompts, %d selected", len(ds.Prompts), len(selected))

	store, err := collect.NewStore(*output)
	if err != nil {
		log.Fatalf("opening store: %v", err)
	}
	log.Printf("already collected: %d", store.Count())

	drafter := client.NewOpenAICompatibleClient(*drafterBaseURL, drafterKey, *drafterModel, "drafter", 10*time.Minute)
	heavyweight := client.NewOpenAICompatibleClient(*heavyBaseURL, heavyKey, *heavyModel, "heavyweight", 120*time.Second)
	judge := collect.NewJudge(heavyweight)

	collector := collect.NewCollector(drafter, heavyweight, judge, collect.CollectorConfig{
		TopLogprobs: *topLogprobs,
		MaxTokens:   *maxTokens,
		DrafterOnly: *drafterOnly,
		DraftModel:  *drafterModel,
		HeavyModel:  *heavyModel,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := store.Count()
	total := len(selected)

	for _, p := range selected {
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
				log.Printf("%s: fatal error: %v", prompt.ID, err)
				return
			}

			if record.Error != "" && isRateLimit(record.Error) {
				for retry := 0; retry < 3; retry++ {
					wait := time.Duration(30*(retry+1)) * time.Second
					log.Printf("%s: rate limited, retrying in %v", prompt.ID, wait)
					select {
					case <-time.After(wait):
					case <-ctx.Done():
						return
					}
					record, err = collector.Collect(ctx, prompt)
					if err != nil {
						log.Printf("%s: retry fatal: %v", prompt.ID, err)
						return
					}
					if record.Error == "" || !isRateLimit(record.Error) {
						break
					}
				}
			}

			if ctx.Err() != nil && record.Error != "" {
				return
			}

			if err := store.Append(record); err != nil {
				log.Printf("%s: store error: %v", prompt.ID, err)
				return
			}

			mu.Lock()
			completed++
			n := completed
			mu.Unlock()

			status := "error: " + record.Error
			if record.Error == "" {
				status = fmt.Sprintf("acceptable=%v, mean_entropy=%.2f", record.Judge.Acceptable, record.MeanEntropy)
				if record.HasGroundTruth() {
					status += fmt.Sprintf(", heavy_correct=%v", *record.HeavyCorrect)
				}
			}
			log.Printf("[%d/%d] %s: %s, %.1fs", n, total, prompt.ID, status,
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

func filterPrompts(prompts []dataset.Prompt, categories string, limit int) []dataset.Prompt {
	allowed := map[string]bool{}
	for _, c := range strings.Split(categories, ",") {
		if c = strings.TrimSpace(c); c != "" {
			allowed[c] = true
		}
	}
	perCategory := map[string]int{}
	var out []dataset.Prompt
	for _, p := range prompts {
		cat := string(p.Category)
		if len(allowed) > 0 && !allowed[cat] {
			continue
		}
		if limit > 0 && perCategory[cat] >= limit {
			continue
		}
		perCategory[cat]++
		out = append(out, p)
	}
	return out
}

func isRateLimit(errStr string) bool {
	return strings.Contains(errStr, "429") || strings.Contains(strings.ToLower(errStr), "rate limit")
}
