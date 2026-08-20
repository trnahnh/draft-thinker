package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/benchmarks/internal/sweep"
)

func main() {
	input := flag.String("input", "benchmarks/results/collected.jsonl", "collected JSONL input path")
	output := flag.String("output", "benchmarks/results/compare.csv", "comparison CSV output path")
	entropyThresholdsStr := flag.String("entropy-thresholds", "1.00,1.25,1.50,1.75,2.00,2.25,2.50", "comma-separated entropy thresholds")
	confidenceThresholdsStr := flag.String("confidence-thresholds", "-0.01,-0.05,-0.1,-0.25,-0.5,-1.0,-2.0,-3.0,-5.0,-8.0", "comma-separated top-1 logprob thresholds")
	windowSize := flag.Int("window-size", 10, "entropy window size")
	earlyExitCount := flag.Int("early-exit-count", 10, "early exit token count")
	bootstrap := flag.Bool("bootstrap", true, "compute bootstrap confidence intervals for the headline thresholds")
	bootstrapIterations := flag.Int("bootstrap-iterations", 2000, "bootstrap resample count")
	bootstrapSeed := flag.Int64("bootstrap-seed", 42, "bootstrap RNG seed")
	headlineEntropyThreshold := flag.Float64("headline-entropy-threshold", 2.0, "entropy threshold to bootstrap (matches the calibrated production threshold in METRICS.md)")
	flag.Parse()

	entropyThresholds, err := parseFloats(*entropyThresholdsStr)
	if err != nil {
		log.Fatalf("parsing entropy thresholds: %v", err)
	}
	confidenceThresholds, err := parseFloats(*confidenceThresholdsStr)
	if err != nil {
		log.Fatalf("parsing confidence thresholds: %v", err)
	}

	records, err := loadRecords(*input)
	if err != nil {
		log.Fatalf("loading records: %v", err)
	}

	var valid []*collect.Record
	var missingLogprobs int
	for _, r := range records {
		if r.Error != "" {
			continue
		}
		valid = append(valid, r)
		if !hasLogprobs(r) {
			missingLogprobs++
		}
	}
	log.Printf("loaded %d records (%d valid)", len(records), len(valid))
	if missingLogprobs > 0 {
		log.Printf("warning: %d/%d valid records have no non-zero token logprob; confidence-threshold results for them will always read as maximally confident (never escalate)", missingLogprobs, len(valid))
	}

	replayCfg := sweep.ReplayConfig{WindowSize: *windowSize, EarlyExitCount: *earlyExitCount}
	pricing := sweep.DefaultPricing()

	var allStats []sweep.ThresholdStats

	for _, t := range entropyThresholds {
		allStats = append(allStats, sweep.ComputeStatsWithPolicy(valid, sweep.MethodEntropyRouter, t, sweep.EntropyPolicy(t, replayCfg), pricing))
	}

	for _, t := range confidenceThresholds {
		allStats = append(allStats, sweep.ComputeStatsWithPolicy(valid, sweep.MethodConfidenceThreshold, t, sweep.ConfidencePolicy(t), pricing))
	}

	allStats = append(allStats, sweep.ComputeStatsWithPolicy(valid, sweep.MethodAlwaysHeavyweight, 0, sweep.AlwaysHeavyweightPolicy(), pricing))

	csvFile, err := os.Create(*output)
	if err != nil {
		log.Fatalf("creating CSV output: %v", err)
	}
	defer csvFile.Close()

	if err := sweep.WriteComparisonCSV(csvFile, allStats); err != nil {
		log.Fatalf("writing CSV: %v", err)
	}
	log.Printf("wrote comparison CSV to %s", *output)

	fmt.Println()
	if err := sweep.WriteComparisonSummary(os.Stdout, allStats); err != nil {
		log.Fatalf("writing summary: %v", err)
	}

	if !*bootstrap {
		return
	}

	best := sweep.ComputeStatsWithPolicy(valid, sweep.MethodEntropyRouter, *headlineEntropyThreshold, sweep.EntropyPolicy(*headlineEntropyThreshold, replayCfg), pricing)

	// Pick the confidence threshold whose cost reduction is closest to the
	// selected entropy threshold's, for an apples-to-apples comparison at
	// matched cost.
	var closestConfidence sweep.ThresholdStats
	bestDelta := -1.0
	for _, s := range allStats {
		if s.Method != sweep.MethodConfidenceThreshold {
			continue
		}
		delta := s.CostReduction - best.CostReduction
		if delta < 0 {
			delta = -delta
		}
		if bestDelta < 0 || delta < bestDelta {
			bestDelta = delta
			closestConfidence = s
		}
	}

	fmt.Println()
	fmt.Printf("Bootstrap 95%% CI (%d resamples, seed=%d):\n", *bootstrapIterations, *bootstrapSeed)

	entropyCI := sweep.Bootstrap(valid, sweep.MethodEntropyRouter, best.Threshold, sweep.EntropyPolicy(best.Threshold, replayCfg), pricing, *bootstrapIterations, *bootstrapSeed)
	fmt.Printf("  entropy_router       T=%.2f  cost_reduction=%.1f%% [%.1f%%, %.1f%%]  accuracy=%.1f%% [%.1f%%, %.1f%%]\n",
		entropyCI.Threshold,
		entropyCI.CostReductionPoint*100, entropyCI.CostReductionLower*100, entropyCI.CostReductionUpper*100,
		entropyCI.AccuracyPoint*100, entropyCI.AccuracyLower*100, entropyCI.AccuracyUpper*100)

	if bestDelta >= 0 {
		confidenceCI := sweep.Bootstrap(valid, sweep.MethodConfidenceThreshold, closestConfidence.Threshold, sweep.ConfidencePolicy(closestConfidence.Threshold), pricing, *bootstrapIterations, *bootstrapSeed)
		fmt.Printf("  confidence_threshold T=%.2f  cost_reduction=%.1f%% [%.1f%%, %.1f%%]  accuracy=%.1f%% [%.1f%%, %.1f%%]  (matched to entropy_router's cost reduction)\n",
			confidenceCI.Threshold,
			confidenceCI.CostReductionPoint*100, confidenceCI.CostReductionLower*100, confidenceCI.CostReductionUpper*100,
			confidenceCI.AccuracyPoint*100, confidenceCI.AccuracyLower*100, confidenceCI.AccuracyUpper*100)
	}
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

func loadRecords(path string) ([]*collect.Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening records: %w", err)
	}
	defer f.Close()

	var records []*collect.Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r collect.Record
		if err := json.Unmarshal(line, &r); err != nil {
			log.Printf("warning: skipping malformed line %d: %v", lineNum, err)
			continue
		}
		records = append(records, &r)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading records: %w", err)
	}

	return records, nil
}
