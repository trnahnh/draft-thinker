// replayspec measures speculative execution offline by feeding recorded
// drafter traces through the production speculative.Executor with a fake
// heavyweight client. Each recorded token becomes a stream chunk whose
// top-5 logprobs are synthesised to reproduce the recorded entropy exactly,
// so the executor sees the same per-token entropies the gateway saw. It
// reports how often the soft threshold fires, how often the speculative
// heavyweight call is cancelled because the drafter recovered, and how
// often it is used, per soft-threshold multiplier.
//
//	go run ./benchmarks/cmd/replayspec -input benchmarks/results/collected.jsonl.gz
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
	"sync/atomic"
	"time"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/benchmarks/internal/sweep"
	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/speculative"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type scenario struct {
	SoftMult             float64 `json:"soft_threshold_mult"`
	Requests             int     `json:"requests"`
	Triggers             int     `json:"speculative_triggers"`
	Cancellations        int     `json:"speculative_cancellations"`
	UsedOnEscalation     int     `json:"speculative_used_on_escalation"`
	EscalatedNoHeadstart int     `json:"escalated_without_speculation"`
	Escalations          int     `json:"escalations"`
	TriggerRate          float64 `json:"trigger_rate"`
	CancelRatio          float64 `json:"cancel_ratio"`
	HeadstartP50MS       float64 `json:"estimated_headstart_p50_ms"`
	HeadstartP90MS       float64 `json:"estimated_headstart_p90_ms"`
	WastedCallsPer1K     float64 `json:"cancelled_heavyweight_calls_per_1k"`
	WasteUpperBoundCost  float64 `json:"waste_upper_bound_share_of_baseline_cost"`
}

type summary struct {
	Input           string     `json:"input"`
	Threshold       float64    `json:"threshold"`
	WindowSize      int        `json:"window_size"`
	EarlyExitCount  int        `json:"early_exit_count"`
	Records         int        `json:"records"`
	ReplayEscalated int        `json:"offline_replay_escalations"`
	Scenarios       []scenario `json:"scenarios"`
}

func main() {
	input := flag.String("input", "benchmarks/results/collected.jsonl.gz", "collected JSONL (optionally .gz) trace")
	output := flag.String("output", "", "JSON summary path (default benchmarks/results/speculative_<trace>.json)")
	threshold := flag.Float64("threshold", 2.0, "hard entropy threshold T")
	windowSize := flag.Int("window-size", 10, "entropy window size")
	earlyExit := flag.Int("early-exit-count", 10, "early exit token count")
	multsStr := flag.String("soft-mults", "0.5,0.6,0.7,0.8,0.9", "soft threshold multipliers to replay (production uses 0.8)")
	flag.Parse()

	if *output == "" {
		base := strings.TrimSuffix(strings.TrimSuffix(filepathBase(*input), ".gz"), ".jsonl")
		*output = "benchmarks/results/speculative_" + strings.TrimPrefix(base, "collected_") + ".json"
		if base == "collected" {
			*output = "benchmarks/results/speculative_project.json"
		}
	}

	records, err := collect.LoadRecords(*input)
	if err != nil {
		log.Fatalf("loading records: %v", err)
	}
	valid := collect.ValidRecords(records)
	log.Printf("loaded %d records (%d valid)", len(records), len(valid))

	replayCfg := sweep.ReplayConfig{WindowSize: *windowSize, EarlyExitCount: *earlyExit}
	replayEsc := 0
	for _, r := range valid {
		if sweep.Replay(r.Tokens, *threshold, replayCfg).WouldEscalate {
			replayEsc++
		}
	}

	sum := summary{Input: *input, Threshold: *threshold, WindowSize: *windowSize, EarlyExitCount: *earlyExit, Records: len(valid), ReplayEscalated: replayEsc}
	pricing := sweep.DefaultPricing()
	baselineTotal := 0.0
	for _, r := range valid {
		baselineTotal += sweep.BaselineCost(r, pricing)
	}

	for _, m := range strings.Split(*multsStr, ",") {
		var mult float64
		if _, err := fmt.Sscanf(strings.TrimSpace(m), "%g", &mult); err != nil {
			log.Fatalf("bad soft multiplier %q", m)
		}
		sc := runScenario(valid, *threshold, mult, replayCfg, pricing, baselineTotal)
		sum.Scenarios = append(sum.Scenarios, sc)
	}

	fmt.Printf("\nSpeculative execution replayed through speculative.Executor (T=%.2f, window=%d, early_exit=%d, n=%d)\n", *threshold, *windowSize, *earlyExit, len(valid))
	fmt.Printf("offline sweep.Replay escalations at T: %d (executor must agree)\n\n", replayEsc)
	fmt.Printf("%-9s %-9s %-8s %-9s %-11s %10s %10s %11s %11s %10s\n", "soft_mult", "triggers", "cancel", "used", "esc_nospec", "trig_rate", "cancel%", "headst_p50", "headst_p90", "waste_ub%")
	for _, sc := range sum.Scenarios {
		fmt.Printf("%-9.2f %-9d %-8d %-9d %-11d %9.1f%% %9.1f%% %11.0f %11.0f %9.1f%%\n",
			sc.SoftMult, sc.Triggers, sc.Cancellations, sc.UsedOnEscalation, sc.EscalatedNoHeadstart,
			sc.TriggerRate*100, sc.CancelRatio*100, sc.HeadstartP50MS, sc.HeadstartP90MS, sc.WasteUpperBoundCost*100)
	}
	fmt.Println("\nheadstart = drafter time between soft trigger and hard escalation, estimated from recorded drafter latency per token.")
	fmt.Println("waste_ub = cost if every cancelled speculative call had run to completion, as a share of always-heavyweight cost.")

	data, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		log.Fatalf("encoding summary: %v", err)
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
		log.Fatalf("writing summary: %v", err)
	}
	log.Printf("wrote %s", *output)
}

func runScenario(records []*collect.Record, threshold, mult float64, replayCfg sweep.ReplayConfig, pricing sweep.Pricing, baselineTotal float64) scenario {
	sc := scenario{SoftMult: mult, Requests: len(records)}
	rec := &tallyRecorder{}
	hw := &fakeHeavyweight{}
	exec := speculative.NewExecutor(speculative.ExecutorConfig{
		WindowCfg:         entropy.WindowConfig{Size: replayCfg.WindowSize, Threshold: threshold, EarlyExitCount: replayCfg.EarlyExitCount},
		SoftThresholdMult: mult,
	}, hw, rec)

	var headstarts []float64
	wasteCost := 0.0
	softThreshold := threshold * mult
	for _, r := range records {
		beforeCancel := rec.cancellations.Load()
		res, err := exec.Execute(context.Background(), drafterStream(r), &protocol.ChatCompletionRequest{})
		if err != nil {
			log.Fatalf("%s: executor error: %v", r.PromptID, err)
		}
		cancelled := rec.cancellations.Load() > beforeCancel
		if res.State == speculative.Escalated {
			sc.Escalations++
			if res.HeavyCh != nil {
				sc.UsedOnEscalation++
				res.HeavyCancel()
				headstarts = append(headstarts, headstartMS(r, softThreshold, res.TokenCount))
			} else {
				sc.EscalatedNoHeadstart++
			}
		}
		if cancelled {
			wasteCost += sweep.BaselineCost(r, pricing)
		}
	}
	sc.Triggers = int(rec.triggers.Load())
	sc.Cancellations = int(rec.cancellations.Load())
	if sc.Requests > 0 {
		sc.TriggerRate = float64(sc.Triggers) / float64(sc.Requests)
		sc.WastedCallsPer1K = float64(sc.Cancellations) / float64(sc.Requests) * 1000
	}
	if sc.Triggers > 0 {
		sc.CancelRatio = float64(sc.Cancellations) / float64(sc.Triggers)
	}
	if baselineTotal > 0 {
		sc.WasteUpperBoundCost = wasteCost / baselineTotal
	}
	if len(headstarts) > 0 {
		sort.Float64s(headstarts)
		sc.HeadstartP50MS = headstarts[len(headstarts)*50/100]
		sc.HeadstartP90MS = headstarts[min(len(headstarts)*90/100, len(headstarts)-1)]
	}
	return sc
}

// headstartMS estimates how long the speculative heavyweight call had been
// running when the hard threshold fired: the drafter tokens between the
// first token above the soft threshold and the escalation token, at the
// record's average drafter milliseconds per token.
func headstartMS(r *collect.Record, softThreshold float64, escalatedAt int) float64 {
	if r.TokenCount == 0 {
		return 0
	}
	trigger := -1
	for i, t := range r.Tokens {
		if t.Entropy > softThreshold {
			trigger = i
			break
		}
	}
	if trigger < 0 {
		return 0
	}
	perToken := float64(r.DraftLatencyMS) / float64(r.TokenCount)
	return float64(escalatedAt-1-trigger) * perToken
}

// drafterStream replays a record as a channel of stream chunks, one token
// per chunk, with synthetic top-5 logprobs that reproduce the recorded
// entropy under entropy.ComputeEntropy.
func drafterStream(r *collect.Record) <-chan client.StreamChunk {
	ch := make(chan client.StreamChunk)
	go func() {
		defer close(ch)
		for _, t := range r.Tokens {
			chunk := &protocol.StreamChunk{Choices: []protocol.Choice{{
				Delta:    &protocol.Delta{Content: t.Token},
				Logprobs: &protocol.ChoiceLogprobs{Content: []protocol.TokenLogprob{{Token: t.Token, Logprob: t.Logprob, TopLogprobs: syntheticTopLogprobs(t)}}},
			}}}
			ch <- client.StreamChunk{Chunk: chunk}
		}
	}()
	return ch
}

// syntheticTopLogprobs builds five alternatives, one with probability p and
// four sharing 1-p, where p is solved so the renormalised Shannon entropy
// equals the recorded value. H(p) is monotone decreasing on [0.2, 1], from
// log2(5) bits down to 0, so bisection recovers p for any valid entropy.
func syntheticTopLogprobs(t collect.TokenRecord) []protocol.TopLogprobEntry {
	target := t.Entropy
	if target <= 1e-12 {
		return []protocol.TopLogprobEntry{{Token: t.Token, Logprob: 0}}
	}
	maxH := math.Log2(5)
	if target >= maxH-1e-12 {
		target = maxH
	}
	lo, hi := 0.2, 1.0
	for range 200 {
		mid := (lo + hi) / 2
		if entropyFor(mid) > target {
			lo = mid
		} else {
			hi = mid
		}
	}
	p := (lo + hi) / 2
	entries := []protocol.TopLogprobEntry{{Token: t.Token, Logprob: math.Log(p)}}
	q := (1 - p) / 4
	for i := range 4 {
		entries = append(entries, protocol.TopLogprobEntry{Token: fmt.Sprintf("<alt%d>", i), Logprob: math.Log(q)})
	}
	return entries
}

func entropyFor(p float64) float64 {
	q := (1 - p) / 4
	h := -p * math.Log2(p)
	if q > 0 {
		h -= 4 * q * math.Log2(q)
	}
	return h
}

type fakeHeavyweight struct{}

func (f *fakeHeavyweight) Complete(ctx context.Context, req *protocol.ChatCompletionRequest) (*protocol.ChatCompletionResponse, error) {
	return nil, fmt.Errorf("not used")
}

// Stream returns a channel that stays open until the context is cancelled,
// which is how a cancelled speculative call behaves from the executor's side.
func (f *fakeHeavyweight) Stream(ctx context.Context, req *protocol.ChatCompletionRequest) (<-chan client.StreamChunk, error) {
	ch := make(chan client.StreamChunk)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

type tallyRecorder struct {
	triggers      atomic.Int64
	cancellations atomic.Int64
}

func (t *tallyRecorder) RecordRequest(string, int)                   {}
func (t *tallyRecorder) RecordUpstreamLatency(string, time.Duration) {}
func (t *tallyRecorder) RecordError(string)                          {}
func (t *tallyRecorder) RecordEntropy(float64)                       {}
func (t *tallyRecorder) RecordRoutingDecision(string)                {}
func (t *tallyRecorder) RecordSpeculativeTrigger()                   { t.triggers.Add(1) }
func (t *tallyRecorder) RecordSpeculativeCancellation()              { t.cancellations.Add(1) }
func (t *tallyRecorder) RecordSpeculativeLatencySaved(time.Duration) {}
func (t *tallyRecorder) RecordCacheHit()                             {}
func (t *tallyRecorder) RecordCacheMiss()                            {}
func (t *tallyRecorder) RecordCacheLookupLatency(time.Duration)      {}

func filepathBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}
