package sweep

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

// FeatureNames is the fixed, ordered feature vector shared by the feature
// exporter (training data for Python) and the learned-router evaluator
// (inference in Go). Both sides compute features with ExtractFeatures, so
// the only thing that crosses the language boundary is the fitted model.
var FeatureNames = []string{
	"mean_entropy",
	"max_entropy",
	"std_entropy",
	"frac_above_0_5",
	"frac_above_1_0",
	"frac_above_1_5",
	"window_peak",
	"last10_mean_entropy",
	"token_count",
	"log_token_count",
	"response_chars",
	"prompt_tokens",
	"min_logprob",
	"mean_logprob",
	"frac_outside_topk",
	"cat_gsm8k",
	"cat_mmlu",
	"cat_arc",
	"cat_simple_factual",
	"cat_multi_step_reasoning",
	"cat_code_generation",
	"cat_ambiguous_creative",
}

// minLogprobFloor clamps the minimum sampled-token logprob so that a single
// very unlikely token does not dominate linear models. Sentinel (-9999)
// tokens are excluded from the logprob features and counted in
// frac_outside_topk instead.
const minLogprobFloor = -20.0

// FeatureConfig controls how much of the drafter trace the features may
// see. PrefixTokens > 0 restricts every trace statistic to the first k
// tokens, which is what an early-exit router would have available; 0 uses
// the whole draft.
type FeatureConfig struct {
	PrefixTokens int
	Replay       ReplayConfig
}

// ExtractFeatures computes the feature vector for one record in the order
// of FeatureNames.
func ExtractFeatures(r *collect.Record, cfg FeatureConfig) []float64 {
	tokens := r.Tokens
	if cfg.PrefixTokens > 0 && len(tokens) > cfg.PrefixTokens {
		tokens = tokens[:cfg.PrefixTokens]
	}
	n := len(tokens)

	var sum, sumSq, maxE float64
	var above05, above10, above15, outside int
	minLP := 0.0
	lpSum := 0.0
	lpCount := 0
	for _, t := range tokens {
		sum += t.Entropy
		sumSq += t.Entropy * t.Entropy
		if t.Entropy > maxE {
			maxE = t.Entropy
		}
		if t.Entropy > 0.5 {
			above05++
		}
		if t.Entropy > 1.0 {
			above10++
		}
		if t.Entropy > 1.5 {
			above15++
		}
		if t.SampledOutsideTopK() {
			outside++
			continue
		}
		lp := math.Max(t.Logprob, minLogprobFloor)
		if lpCount == 0 || lp < minLP {
			minLP = lp
		}
		lpSum += lp
		lpCount++
	}

	mean, std := 0.0, 0.0
	if n > 0 {
		mean = sum / float64(n)
		variance := sumSq/float64(n) - mean*mean
		if variance > 0 {
			std = math.Sqrt(variance)
		}
	}
	frac := func(k int) float64 {
		if n == 0 {
			return 0
		}
		return float64(k) / float64(n)
	}
	meanLP := 0.0
	if lpCount > 0 {
		meanLP = lpSum / float64(lpCount)
	}

	last10 := 0.0
	if n > 0 {
		start := max(n-10, 0)
		s := 0.0
		for _, t := range tokens[start:] {
			s += t.Entropy
		}
		last10 = s / float64(n-start)
	}

	responseChars := len(r.DraftResponse)
	if cfg.PrefixTokens > 0 && len(r.Tokens) > cfg.PrefixTokens {
		chars := 0
		for _, t := range tokens {
			chars += len(t.Token)
		}
		responseChars = chars
	}

	cat := func(name string) float64 {
		if r.Category == name {
			return 1
		}
		return 0
	}

	return []float64{
		mean,
		maxE,
		std,
		frac(above05),
		frac(above10),
		frac(above15),
		windowPeak(tokens, cfg.Replay),
		last10,
		float64(n),
		math.Log1p(float64(n)),
		float64(responseChars),
		float64(r.EstimatedPromptTokens()),
		minLP,
		meanLP,
		frac(outside),
		cat("gsm8k"),
		cat("mmlu"),
		cat("arc"),
		cat("simple_factual"),
		cat("multi_step_reasoning"),
		cat("code_generation"),
		cat("ambiguous_creative"),
	}
}

func windowPeak(tokens []collect.TokenRecord, cfg ReplayConfig) float64 {
	if cfg.WindowSize <= 0 {
		cfg = ReplayConfig{WindowSize: 10, EarlyExitCount: 10}
	}
	rec := &collect.Record{Tokens: tokens}
	return EntropyScore(cfg)(rec)
}

// WriteFeaturesCSV writes one row per valid record: identifiers, the
// label (1 when the draft was unacceptable, so a positive means "should
// have escalated") and the feature vector.
func WriteFeaturesCSV(w io.Writer, records []*collect.Record, source string, cfg FeatureConfig) error {
	cw := csv.NewWriter(w)
	header := append([]string{"source", "prompt_id", "category", "label", "heavy_correct"}, FeatureNames...)
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	cw.Flush()
	return AppendFeaturesCSV(w, records, source, cfg)
}

// AppendFeaturesCSV writes feature rows without a header, for pooling
// several traces into one file.
func AppendFeaturesCSV(w io.Writer, records []*collect.Record, source string, cfg FeatureConfig) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	for _, r := range records {
		if r.Error != "" {
			continue
		}
		label := "0"
		if !r.Judge.Acceptable {
			label = "1"
		}
		heavy := ""
		if r.HeavyCorrect != nil {
			heavy = "0"
			if *r.HeavyCorrect {
				heavy = "1"
			}
		}
		row := []string{source, r.PromptID, r.Category, label, heavy}
		for _, v := range ExtractFeatures(r, cfg) {
			row = append(row, strconv.FormatFloat(v, 'g', -1, 64))
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing row: %w", err)
		}
	}
	return nil
}
