package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/benchmarks/internal/sweep"
)

type ciSummary struct {
	Method             string  `json:"method"`
	Threshold          float64 `json:"threshold"`
	EscalationRate     float64 `json:"escalation_rate"`
	OverallAccuracy    float64 `json:"overall_accuracy"`
	AccuracyLower      float64 `json:"accuracy_ci_lower"`
	AccuracyUpper      float64 `json:"accuracy_ci_upper"`
	CostReduction      float64 `json:"cost_reduction"`
	CostReductionLower float64 `json:"cost_reduction_ci_lower"`
	CostReductionUpper float64 `json:"cost_reduction_ci_upper"`
	CostPer1K          float64 `json:"cost_per_1k_usd"`
	LatencyP50MS       float64 `json:"latency_p50_ms"`
	LatencyP99MS       float64 `json:"latency_p99_ms"`
	TP                 int     `json:"tp"`
	FP                 int     `json:"fp"`
	TN                 int     `json:"tn"`
	FN                 int     `json:"fn"`
}

type aurocSummary struct {
	Score     string  `json:"score"`
	AUROC     float64 `json:"auroc"`
	Lower     float64 `json:"ci_lower"`
	Upper     float64 `json:"ci_upper"`
	Positives int     `json:"positives"`
	Negatives int     `json:"negatives"`
}

type artifactSummary struct {
	Category          string  `json:"category"`
	Records           int     `json:"records"`
	RecordsWithEvent  int     `json:"records_with_outside_top_k"`
	RecordFraction    float64 `json:"record_fraction"`
	Tokens            int     `json:"tokens"`
	TokensOutsideTopK int     `json:"tokens_outside_top_k"`
	TokenFraction     float64 `json:"token_fraction"`
}

type summary struct {
	Input               string            `json:"input"`
	Records             int               `json:"records"`
	Valid               int               `json:"valid"`
	UnacceptableDrafts  int               `json:"unacceptable_drafts"`
	GroundTruthRecords  int               `json:"ground_truth_records"`
	HeavyWrong          int               `json:"heavyweight_wrong"`
	Pricing             string            `json:"pricing"`
	BaselineCostPer1K   float64           `json:"baseline_cost_per_1k_usd"`
	BootstrapIterations int               `json:"bootstrap_iterations"`
	BootstrapSeed       int64             `json:"bootstrap_seed"`
	Headline            []ciSummary       `json:"headline"`
	AUROC               []aurocSummary    `json:"auroc"`
	Artifact            []artifactSummary `json:"logprob_artifact"`
}

func main() {
	input := flag.String("input", "benchmarks/results/collected.jsonl.gz", "collected JSONL (optionally .gz) input path")
	output := flag.String("output", "benchmarks/results/compare.csv", "comparison CSV output path")
	categoryOutput := flag.String("category-output", "", "per-category CSV output path (default: <output> with _by_category suffix)")
	summaryOutput := flag.String("summary-output", "", "JSON summary output path (default: <output> with .json extension)")
	entropyThresholdsStr := flag.String("entropy-thresholds", "1.00,1.25,1.50,1.75,2.00,2.25,2.50", "comma-separated entropy thresholds")
	confidenceThresholdsStr := flag.String("confidence-thresholds", "-0.01,-0.05,-0.1,-0.25,-0.5,-1.0,-2.0,-3.0,-5.0,-8.0,-10.0,-12.0,-15.0,-20.0,-40.0", "comma-separated min-logprob thresholds")
	windowSize := flag.Int("window-size", 10, "entropy window size")
	earlyExitCount := flag.Int("early-exit-count", 10, "early exit token count")
	bootstrap := flag.Bool("bootstrap", true, "compute bootstrap confidence intervals for the headline thresholds")
	bootstrapIterations := flag.Int("bootstrap-iterations", 2000, "bootstrap resample count")
	bootstrapSeed := flag.Int64("bootstrap-seed", 42, "bootstrap RNG seed")
	headlineEntropyThreshold := flag.Float64("headline-entropy-threshold", 2.0, "entropy threshold to bootstrap (the calibrated production threshold)")
	randomSeed := flag.Int64("random-seed", 42, "seed for the rate-matched random baseline")
	outputOnlyPricing := flag.Bool("output-only-pricing", false, "ignore input tokens (reproduces the original calibration's cost model)")
	flag.Parse()

	entropyThresholds, err := parseFloats(*entropyThresholdsStr)
	if err != nil {
		log.Fatalf("parsing entropy thresholds: %v", err)
	}
	confidenceThresholds, err := parseFloats(*confidenceThresholdsStr)
	if err != nil {
		log.Fatalf("parsing confidence thresholds: %v", err)
	}
	if *categoryOutput == "" {
		*categoryOutput = strings.TrimSuffix(*output, ".csv") + "_by_category.csv"
	}
	if *summaryOutput == "" {
		*summaryOutput = strings.TrimSuffix(*output, ".csv") + ".json"
	}

	records, err := collect.LoadRecords(*input)
	if err != nil {
		log.Fatalf("loading records: %v", err)
	}
	valid := collect.ValidRecords(records)
	log.Printf("loaded %d records (%d valid)", len(records), len(valid))

	sum := summary{
		Input:               *input,
		Records:             len(records),
		Valid:               len(valid),
		BootstrapIterations: *bootstrapIterations,
		BootstrapSeed:       *bootstrapSeed,
		Pricing:             "input+output",
	}
	var missingLogprobs int
	for _, r := range valid {
		if !r.Judge.Acceptable {
			sum.UnacceptableDrafts++
		}
		if r.HasGroundTruth() {
			sum.GroundTruthRecords++
			if !*r.HeavyCorrect {
				sum.HeavyWrong++
			}
		}
		if !hasLogprobs(r) {
			missingLogprobs++
		}
	}
	if missingLogprobs > 0 {
		log.Printf("warning: %d/%d valid records have no non-zero token logprob; confidence-threshold results for them will always read as maximally confident", missingLogprobs, len(valid))
	}
	log.Printf("unacceptable drafts: %d/%d; ground-truth graded: %d (heavyweight wrong on %d)", sum.UnacceptableDrafts, len(valid), sum.GroundTruthRecords, sum.HeavyWrong)

	replayCfg := sweep.ReplayConfig{WindowSize: *windowSize, EarlyExitCount: *earlyExitCount}
	pricing := sweep.DefaultPricing()
	if *outputOnlyPricing {
		pricing = pricing.OutputOnly()
		sum.Pricing = "output-only"
	}

	var allStats []sweep.ThresholdStats
	for _, t := range entropyThresholds {
		allStats = append(allStats, sweep.ComputeStatsWithPolicy(valid, sweep.MethodEntropyRouter, t, sweep.EntropyPolicy(t, replayCfg), pricing))
	}
	for _, t := range confidenceThresholds {
		allStats = append(allStats, sweep.ComputeStatsWithPolicy(valid, sweep.MethodConfidenceThreshold, t, sweep.ConfidencePolicy(t), pricing))
	}

	headline := sweep.ComputeStatsWithPolicy(valid, sweep.MethodEntropyRouter, *headlineEntropyThreshold, sweep.EntropyPolicy(*headlineEntropyThreshold, replayCfg), pricing)
	randomPolicy := sweep.RandomPolicy(headline.EscalationRate, *randomSeed)
	randomStats := sweep.ComputeStatsWithPolicy(valid, sweep.MethodRandom, headline.EscalationRate, randomPolicy, pricing)
	drafterStats := sweep.ComputeStatsWithPolicy(valid, sweep.MethodAlwaysDrafter, 0, sweep.AlwaysDrafterPolicy(), pricing)
	heavyStats := sweep.ComputeStatsWithPolicy(valid, sweep.MethodAlwaysHeavyweight, 0, sweep.AlwaysHeavyweightPolicy(), pricing)
	oracleStats := sweep.ComputeStatsWithPolicy(valid, sweep.MethodOracle, 0, sweep.OraclePolicy(), pricing)
	meanThreshold := sweep.ThresholdForRate(valid, sweep.MeanEntropyScore, headline.EscalationRate)
	meanPolicy := sweep.MeanEntropyPolicy(meanThreshold)
	meanStats := sweep.ComputeStatsWithPolicy(valid, sweep.MethodMeanEntropy, meanThreshold, meanPolicy, pricing)
	allStats = append(allStats, meanStats, randomStats, drafterStats, heavyStats, oracleStats)
	sum.BaselineCostPer1K = heavyStats.BaselineCostPer1K

	if err := writeCSV(*output, func(f *os.File) error { return sweep.WriteComparisonCSV(f, allStats) }); err != nil {
		log.Fatalf("writing CSV: %v", err)
	}
	if err := writeCSV(*categoryOutput, func(f *os.File) error { return sweep.WriteCategoryCSV(f, allStats) }); err != nil {
		log.Fatalf("writing category CSV: %v", err)
	}
	log.Printf("wrote %s and %s", *output, *categoryOutput)

	fmt.Println()
	if err := sweep.WriteComparisonSummary(os.Stdout, allStats); err != nil {
		log.Fatalf("writing summary: %v", err)
	}

	fmt.Println()
	fmt.Println("Logprob artifact (sampled token outside requested top-k) by category:")
	sum.Artifact = artifactStats(valid)
	for _, a := range sum.Artifact {
		fmt.Printf("  %-22s records %3d/%3d (%5.1f%%)  tokens %5d/%6d (%5.2f%%)\n",
			a.Category, a.RecordsWithEvent, a.Records, a.RecordFraction*100, a.TokensOutsideTopK, a.Tokens, a.TokenFraction*100)
	}

	fmt.Println()
	fmt.Printf("AUROC of routing scores as detectors of unacceptable drafts (%d resamples, seed=%d):\n", *bootstrapIterations, *bootstrapSeed)
	scores := []struct {
		name  string
		score sweep.ScoreFunc
	}{
		{"entropy_window_peak", sweep.EntropyScore(replayCfg)},
		{"mean_entropy", sweep.MeanEntropyScore},
		{"frac_tokens_above_1bit", sweep.HighEntropyFraction(1.0)},
		{"neg_min_logprob", sweep.MinLogprobScore},
	}
	for _, s := range scores {
		res := sweep.BootstrapAUROC(valid, s.name, s.score, *bootstrapIterations, *bootstrapSeed)
		if !res.OK {
			fmt.Printf("  %-22s n/a (needs both acceptable and unacceptable drafts)\n", s.name)
			continue
		}
		fmt.Printf("  %-22s AUROC=%.3f [%.3f, %.3f]  (positives=%d, negatives=%d)\n", s.name, res.Point, res.Lower, res.Upper, res.Positives, res.Negatives)
		sum.AUROC = append(sum.AUROC, aurocSummary{Score: s.name, AUROC: res.Point, Lower: res.Lower, Upper: res.Upper, Positives: res.Positives, Negatives: res.Negatives})
	}

	if *bootstrap {
		closest := closestConfidence(allStats, headline.CostReduction)

		fmt.Println()
		fmt.Printf("Bootstrap 95%% CI (%d resamples, seed=%d):\n", *bootstrapIterations, *bootstrapSeed)
		runs := []struct {
			stats  sweep.ThresholdStats
			policy sweep.EscalationPolicy
			note   string
		}{
			{headline, sweep.EntropyPolicy(headline.Threshold, replayCfg), "calibrated production threshold"},
			{closest, sweep.ConfidencePolicy(closest.Threshold), "closest cost reduction to entropy_router"},
			{meanStats, meanPolicy, "trace-mean entropy, threshold matched to entropy_router escalation rate"},
			{randomStats, randomPolicy, "same escalation rate as entropy_router, chosen blind"},
			{drafterStats, sweep.AlwaysDrafterPolicy(), "cost floor"},
			{heavyStats, sweep.AlwaysHeavyweightPolicy(), "cost ceiling"},
			{oracleStats, sweep.OraclePolicy(), "perfect label knowledge, unattainable"},
		}
		for _, run := range runs {
			ci := sweep.Bootstrap(valid, run.stats.Method, run.stats.Threshold, run.policy, pricing, *bootstrapIterations, *bootstrapSeed)
			fmt.Printf("  %-22s T=%-7.2f cost_reduction=%5.1f%% [%5.1f%%, %5.1f%%]  accuracy=%5.1f%% [%5.1f%%, %5.1f%%]  %s\n",
				run.stats.Method, run.stats.Threshold,
				ci.CostReductionPoint*100, ci.CostReductionLower*100, ci.CostReductionUpper*100,
				ci.AccuracyPoint*100, ci.AccuracyLower*100, ci.AccuracyUpper*100, run.note)
			sum.Headline = append(sum.Headline, ciSummary{
				Method: string(run.stats.Method), Threshold: run.stats.Threshold,
				EscalationRate:  run.stats.EscalationRate,
				OverallAccuracy: ci.AccuracyPoint, AccuracyLower: ci.AccuracyLower, AccuracyUpper: ci.AccuracyUpper,
				CostReduction: ci.CostReductionPoint, CostReductionLower: ci.CostReductionLower, CostReductionUpper: ci.CostReductionUpper,
				CostPer1K: run.stats.CostPer1K, LatencyP50MS: run.stats.LatencyP50MS, LatencyP99MS: run.stats.LatencyP99MS,
				TP: run.stats.TP, FP: run.stats.FP, TN: run.stats.TN, FN: run.stats.FN,
			})
		}
	}

	data, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		log.Fatalf("encoding summary: %v", err)
	}
	if err := os.WriteFile(*summaryOutput, append(data, '\n'), 0644); err != nil {
		log.Fatalf("writing summary JSON: %v", err)
	}
	log.Printf("wrote %s", *summaryOutput)
}

func closestConfidence(all []sweep.ThresholdStats, target float64) sweep.ThresholdStats {
	var best sweep.ThresholdStats
	bestDelta := -1.0
	for _, s := range all {
		if s.Method != sweep.MethodConfidenceThreshold {
			continue
		}
		delta := s.CostReduction - target
		if delta < 0 {
			delta = -delta
		}
		if bestDelta < 0 || delta < bestDelta {
			bestDelta = delta
			best = s
		}
	}
	return best
}

func artifactStats(records []*collect.Record) []artifactSummary {
	byCat := map[string]*artifactSummary{}
	for _, r := range records {
		a, ok := byCat[r.Category]
		if !ok {
			a = &artifactSummary{Category: r.Category}
			byCat[r.Category] = a
		}
		a.Records++
		hit := false
		for _, t := range r.Tokens {
			a.Tokens++
			if t.SampledOutsideTopK() {
				a.TokensOutsideTopK++
				hit = true
			}
		}
		if hit {
			a.RecordsWithEvent++
		}
	}
	var out []artifactSummary
	for _, a := range byCat {
		if a.Records > 0 {
			a.RecordFraction = float64(a.RecordsWithEvent) / float64(a.Records)
		}
		if a.Tokens > 0 {
			a.TokenFraction = float64(a.TokensOutsideTopK) / float64(a.Tokens)
		}
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Category < out[j].Category })
	return out
}

func writeCSV(path string, write func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return write(f)
}

func hasLogprobs(r *collect.Record) bool {
	for _, t := range r.Tokens {
		if t.Logprob != 0 {
			return true
		}
	}
	return false
}

func parseFloats(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	vals := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q: %w", p, err)
		}
		vals = append(vals, v)
	}
	if len(vals) == 0 {
		return nil, fmt.Errorf("no values specified")
	}
	return vals, nil
}
