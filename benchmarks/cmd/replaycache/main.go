// replaycache measures the semantic cache on replayed benchmark traffic. It
// drives the production cache.VectorStore (same key text, same embedding
// client, same similarity threshold) with an in-memory vector index and
// key-value store standing in for Qdrant and Redis, replays a trace's
// prompts in order, inserts drafts the entropy router would have accepted
// (the gateway caches only accepted drafts) and reports the hit rate, how
// often a hit served an answer that was wrong for the new prompt, and the
// nearest-neighbour similarity distribution so other thresholds can be
// judged. Embedding calls cost real money but very little (about one cent
// per thousand prompts).
//
//	OPENAI_API_KEY=... go run ./benchmarks/cmd/replaycache -input benchmarks/results/collected_gsm8k.jsonl.gz
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/benchmarks/internal/sweep"
	"github.com/trnahnh/draft-thinker/internal/cache"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type summary struct {
	Input              string             `json:"input"`
	Records            int                `json:"records"`
	Threshold          float64            `json:"similarity_threshold"`
	EmbeddingModel     string             `json:"embedding_model"`
	Inserted           int                `json:"inserted_accepted_drafts"`
	Hits               int                `json:"hits"`
	HitRate            float64            `json:"hit_rate"`
	HitsWrongForPrompt int                `json:"hits_wrong_for_new_prompt"`
	HitsGradable       int                `json:"hits_with_ground_truth"`
	NearestSimilarity  map[string]float64 `json:"nearest_similarity_quantiles"`
	CountAbove         map[string]int     `json:"prompts_with_neighbour_above"`
	WrongAbove         map[string]int     `json:"wrong_answer_hits_if_threshold_were"`
	LookupP50MS        float64            `json:"lookup_p50_ms"`
	LookupP99MS        float64            `json:"lookup_p99_ms"`
	RepeatPassHits     int                `json:"repeat_pass_hits,omitempty"`
	RepeatPassExpected int                `json:"repeat_pass_expected_hits,omitempty"`
	HitExamples        []hitExample       `json:"hit_examples,omitempty"`
}

type hitExample struct {
	PromptID       string  `json:"prompt_id"`
	SourcePromptID string  `json:"source_prompt_id"`
	Score          float64 `json:"score"`
	CorrectForNew  *bool   `json:"cached_answer_correct_for_new_prompt,omitempty"`
}

func main() {
	input := flag.String("input", "benchmarks/results/collected.jsonl.gz", "collected JSONL (optionally .gz) trace")
	output := flag.String("output", "", "JSON summary path (default benchmarks/results/cache_replay_<trace>.json)")
	threshold := flag.Float64("threshold", 0.95, "cosine similarity threshold (production: 0.95)")
	entropyThreshold := flag.Float64("entropy-threshold", 2.0, "entropy threshold deciding which drafts were accepted and therefore cached")
	embeddingModel := flag.String("embedding-model", "text-embedding-3-small", "embedding model")
	dimensions := flag.Int("embedding-dimensions", 1536, "embedding dimensions")
	baseURL := flag.String("base-url", "https://api.openai.com/v1", "OpenAI-compatible base URL for embeddings")
	repeat := flag.Bool("repeat", false, "replay the traffic a second time to confirm exact repeats hit")
	flag.Parse()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required for embeddings")
	}
	if *output == "" {
		base := strings.TrimSuffix(strings.TrimSuffix(baseName(*input), ".gz"), ".jsonl")
		name := strings.TrimPrefix(base, "collected_")
		if base == "collected" {
			name = "project"
		}
		*output = "benchmarks/results/cache_replay_" + name + ".json"
	}

	records, err := collect.LoadRecords(*input)
	if err != nil {
		log.Fatalf("loading records: %v", err)
	}
	valid := collect.ValidRecords(records)
	log.Printf("loaded %d records (%d valid)", len(records), len(valid))

	emb := client.NewEmbeddingClient(*baseURL, apiKey, *embeddingModel, *dimensions, 30*time.Second)
	idx := &memoryIndex{}
	kv := &memoryKV{data: map[string][]byte{}}
	store := cache.NewVectorStore(emb, idx, kv, *threshold, noopRecorder{})

	byID := map[string]*collect.Record{}
	for _, r := range valid {
		byID[r.PromptID] = r
	}
	replayCfg := sweep.ReplayConfig{WindowSize: 10, EarlyExitCount: 10}
	sum := summary{Input: *input, Records: len(valid), Threshold: *threshold, EmbeddingModel: *embeddingModel,
		NearestSimilarity: map[string]float64{}, CountAbove: map[string]int{}, WrongAbove: map[string]int{}}

	ctx := context.Background()
	var nearest []float64
	var lookups []float64
	type neighbour struct {
		score  float64
		wrong  bool
		graded bool
	}
	var neighbours []neighbour
	for i, r := range valid {
		messages := []protocol.Message{{Role: "user", Content: r.PromptText}}
		start := time.Now()
		resp, err := lookupWithRetry(ctx, store, messages)
		if err != nil {
			log.Fatalf("%s: lookup: %v", r.PromptID, err)
		}
		lookups = append(lookups, float64(time.Since(start).Microseconds())/1000)
		best := idx.lastBest
		if best.id != "" {
			nearest = append(nearest, best.score)
			src := byID[best.payload["prompt_id"]]
			nb := neighbour{score: best.score}
			if src != nil && r.HasGroundTruth() && src.HasGroundTruth() {
				_, ok := collect.Grade(r.Grading, src.DraftResponse, r.GroundTruth)
				nb.graded = true
				nb.wrong = !ok
			}
			neighbours = append(neighbours, nb)
		}
		if resp != nil {
			sum.Hits++
			ex := hitExample{PromptID: r.PromptID, SourcePromptID: resp.ID, Score: best.score}
			if src := byID[resp.ID]; src != nil && r.HasGroundTruth() {
				_, ok := collect.Grade(r.Grading, src.DraftResponse, r.GroundTruth)
				ex.CorrectForNew = &ok
				sum.HitsGradable++
				if !ok {
					sum.HitsWrongForPrompt++
				}
			}
			if len(sum.HitExamples) < 20 {
				sum.HitExamples = append(sum.HitExamples, ex)
			}
			continue
		}
		if !sweep.Replay(r.Tokens, *entropyThreshold, replayCfg).WouldEscalate {
			cached := &protocol.ChatCompletionResponse{ID: r.PromptID, Object: "chat.completion", Choices: []protocol.Choice{{Message: &protocol.Message{Role: "assistant", Content: r.DraftResponse}}}}
			if err := insertWithRetry(ctx, store, messages, cached); err != nil {
				log.Fatalf("%s: insert: %v", r.PromptID, err)
			}
			idx.tagLast(r.PromptID)
			sum.Inserted++
		}
		if (i+1)%100 == 0 {
			log.Printf("replayed %d/%d: hits=%d inserted=%d", i+1, len(valid), sum.Hits, sum.Inserted)
		}
	}
	if len(valid) > 0 {
		sum.HitRate = float64(sum.Hits) / float64(len(valid))
	}
	sort.Float64s(nearest)
	for _, q := range []float64{50, 90, 99, 100} {
		sum.NearestSimilarity[fmt.Sprintf("p%g", q)] = quantile(nearest, q)
	}
	for _, th := range []float64{0.80, 0.85, 0.90, 0.95} {
		key := fmt.Sprintf("%.2f", th)
		for _, nb := range neighbours {
			if nb.score >= th {
				sum.CountAbove[key]++
				if nb.graded && nb.wrong {
					sum.WrongAbove[key]++
				}
			}
		}
	}
	sort.Float64s(lookups)
	sum.LookupP50MS = quantile(lookups, 50)
	sum.LookupP99MS = quantile(lookups, 99)

	if *repeat {
		sum.RepeatPassExpected = sum.Inserted
		for _, r := range valid {
			resp, err := lookupWithRetry(ctx, store, []protocol.Message{{Role: "user", Content: r.PromptText}})
			if err != nil {
				log.Fatalf("%s: repeat lookup: %v", r.PromptID, err)
			}
			if resp != nil {
				sum.RepeatPassHits++
			}
		}
	}

	fmt.Printf("\nSemantic cache replay (threshold %.2f, %s, n=%d)\n", *threshold, *embeddingModel, len(valid))
	fmt.Printf("  inserted accepted drafts: %d\n", sum.Inserted)
	fmt.Printf("  hits: %d (%.2f%%), of which wrong for the new prompt: %d of %d gradable\n", sum.Hits, sum.HitRate*100, sum.HitsWrongForPrompt, sum.HitsGradable)
	fmt.Printf("  nearest-neighbour similarity: p50 %.3f, p90 %.3f, p99 %.3f, max %.3f\n", sum.NearestSimilarity["p50"], sum.NearestSimilarity["p90"], sum.NearestSimilarity["p99"], sum.NearestSimilarity["p100"])
	for _, th := range []string{"0.80", "0.85", "0.90", "0.95"} {
		fmt.Printf("  threshold %s would hit %d prompts, serving a wrong cached answer on %d\n", th, sum.CountAbove[th], sum.WrongAbove[th])
	}
	fmt.Printf("  lookup latency (embedding call plus brute-force search): p50 %.0f ms, p99 %.0f ms\n", sum.LookupP50MS, sum.LookupP99MS)
	if *repeat {
		fmt.Printf("  repeat pass: %d hits of %d expected\n", sum.RepeatPassHits, sum.RepeatPassExpected)
	}

	data, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		log.Fatalf("encoding summary: %v", err)
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
		log.Fatalf("writing summary: %v", err)
	}
	log.Printf("wrote %s", *output)
}

// The embedding API occasionally times out; the gateway treats that as a
// cache miss, but a replay needs every lookup to complete, so retry.
func lookupWithRetry(ctx context.Context, store *cache.VectorStore, messages []protocol.Message) (*protocol.ChatCompletionResponse, error) {
	var resp *protocol.ChatCompletionResponse
	var err error
	for attempt := range 4 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * 2 * time.Second)
		}
		resp, err = store.Lookup(ctx, messages)
		if err == nil {
			return resp, nil
		}
		log.Printf("lookup attempt %d failed: %v", attempt+1, err)
	}
	return nil, err
}

func insertWithRetry(ctx context.Context, store *cache.VectorStore, messages []protocol.Message, resp *protocol.ChatCompletionResponse) error {
	var err error
	for attempt := range 4 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * 2 * time.Second)
		}
		if err = store.Insert(ctx, messages, resp); err == nil {
			return nil
		}
		log.Printf("insert attempt %d failed: %v", attempt+1, err)
	}
	return err
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Round(q / 100 * float64(len(sorted)-1)))
	return sorted[min(max(i, 0), len(sorted)-1)]
}

type point struct {
	id      string
	vector  []float64
	payload map[string]string
}

type best struct {
	id      string
	score   float64
	payload map[string]string
}

// memoryIndex is a brute-force cosine index implementing the cache's
// vector index interface. Vectors from the embedding API are unit length,
// so the dot product is the cosine similarity Qdrant would report.
type memoryIndex struct {
	points   []point
	lastBest best
}

func (m *memoryIndex) EnsureCollection(ctx context.Context, dimensions int) error { return nil }

func (m *memoryIndex) Search(ctx context.Context, vector []float64, threshold float64) (*cache.SearchResult, error) {
	m.lastBest = best{}
	for _, p := range m.points {
		s := cosine(vector, p.vector)
		if m.lastBest.id == "" || s > m.lastBest.score {
			m.lastBest = best{id: p.id, score: s, payload: p.payload}
		}
	}
	if m.lastBest.id == "" || m.lastBest.score < threshold {
		return nil, nil
	}
	return &cache.SearchResult{ID: m.lastBest.id, Score: m.lastBest.score, Payload: m.lastBest.payload}, nil
}

func (m *memoryIndex) Upsert(ctx context.Context, id string, vector []float64, payload map[string]string) error {
	cp := map[string]string{}
	for k, v := range payload {
		cp[k] = v
	}
	m.points = append(m.points, point{id: id, vector: vector, payload: cp})
	return nil
}

func (m *memoryIndex) Delete(ctx context.Context, ids []string) error {
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	kept := m.points[:0]
	for _, p := range m.points {
		if !drop[p.id] {
			kept = append(kept, p)
		}
	}
	m.points = kept
	return nil
}

// tagLast records which prompt produced the most recently inserted point,
// so hits can be traced back to their source record.
func (m *memoryIndex) tagLast(promptID string) {
	if len(m.points) > 0 {
		m.points[len(m.points)-1].payload["prompt_id"] = promptID
	}
}

func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

type memoryKV struct{ data map[string][]byte }

func (k *memoryKV) Set(ctx context.Context, key string, data []byte) error {
	k.data[key] = data
	return nil
}
func (k *memoryKV) Get(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok := k.data[key]
	return v, ok, nil
}
func (k *memoryKV) Del(ctx context.Context, key string) error { delete(k.data, key); return nil }
func (k *memoryKV) Close() error                              { return nil }

type noopRecorder struct{}

func (noopRecorder) RecordRequest(string, int)                   {}
func (noopRecorder) RecordUpstreamLatency(string, time.Duration) {}
func (noopRecorder) RecordError(string)                          {}
func (noopRecorder) RecordEntropy(float64)                       {}
func (noopRecorder) RecordRoutingDecision(string)                {}
func (noopRecorder) RecordSpeculativeTrigger()                   {}
func (noopRecorder) RecordSpeculativeCancellation()              {}
func (noopRecorder) RecordSpeculativeLatencySaved(time.Duration) {}
func (noopRecorder) RecordCacheHit()                             {}
func (noopRecorder) RecordCacheMiss()                            {}
func (noopRecorder) RecordCacheLookupLatency(time.Duration)      {}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}
