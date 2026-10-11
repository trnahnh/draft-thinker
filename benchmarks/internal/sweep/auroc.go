package sweep

import (
	"math/rand"
	"sort"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/internal/entropy"
)

// ScoreFunc maps a record to a scalar where higher means "more likely to
// need escalation". Thresholding a ScoreFunc yields a policy; AUROC
// evaluates the score itself, independent of any threshold.
type ScoreFunc func(r *collect.Record) float64

// EntropyScore returns the peak routing statistic the production window
// would have seen: the maximum per-token entropy over the early-exit
// prefix and the maximum windowed average afterwards. A record escalates
// at threshold T exactly when EntropyScore > T, so this is the threshold-
// free form of the entropy router.
func EntropyScore(cfg ReplayConfig) ScoreFunc {
	return func(r *collect.Record) float64 {
		window := entropy.NewWindow(entropy.WindowConfig{Size: cfg.WindowSize, EarlyExitCount: cfg.EarlyExitCount})
		peak := 0.0
		for i, t := range r.Tokens {
			window.Add(t.Entropy)
			if i < cfg.EarlyExitCount && t.Entropy > peak {
				peak = t.Entropy
			}
			if i+1 >= cfg.WindowSize && window.Average() > peak {
				peak = window.Average()
			}
		}
		return peak
	}
}

// MinLogprobScore is the negated minimum sampled-token logprob, so that
// higher means less confident, matching the orientation of EntropyScore.
func MinLogprobScore(r *collect.Record) float64 {
	if len(r.Tokens) == 0 {
		return 0
	}
	minLP := r.Tokens[0].Logprob
	for _, t := range r.Tokens {
		if t.Logprob < minLP {
			minLP = t.Logprob
		}
	}
	return -minLP
}

// MeanEntropyScore is the trace-wide mean per-token entropy, a simpler
// statistic than the windowed peak.
func MeanEntropyScore(r *collect.Record) float64 {
	return r.MeanEntropy
}

// HighEntropyFraction is the share of tokens whose entropy exceeds the
// given number of bits, a measure of how much of the response was
// generated under uncertainty rather than how uncertain the worst moment was.
func HighEntropyFraction(bits float64) ScoreFunc {
	return func(r *collect.Record) float64 {
		if len(r.Tokens) == 0 {
			return 0
		}
		n := 0
		for _, t := range r.Tokens {
			if t.Entropy > bits {
				n++
			}
		}
		return float64(n) / float64(len(r.Tokens))
	}
}

// ThresholdForRate returns the score threshold at which escalating when
// score > threshold yields approximately the requested escalation rate on
// the given records. It is the (1-rate) quantile of the score, which makes
// rate-matched comparisons between scores deterministic.
func ThresholdForRate(records []*collect.Record, score ScoreFunc, rate float64) float64 {
	vals := make([]float64, 0, len(records))
	for _, r := range records {
		if r.Error == "" {
			vals = append(vals, score(r))
		}
	}
	if len(vals) == 0 {
		return 0
	}
	sort.Float64s(vals)
	k := int(float64(len(vals)) * (1 - rate))
	if k >= len(vals) {
		k = len(vals) - 1
	}
	if k < 0 {
		k = 0
	}
	return vals[k]
}

// AUROC computes the area under the ROC curve for score as a detector of
// unacceptable drafts (the positive class). Ties are counted as half.
// 0.5 means the score carries no information about draft quality.
// Returns ok=false when either class is empty.
func AUROC(records []*collect.Record, score ScoreFunc) (float64, bool) {
	type scored struct {
		s   float64
		pos bool
	}
	items := make([]scored, 0, len(records))
	pos, neg := 0, 0
	for _, r := range records {
		if r.Error != "" {
			continue
		}
		p := !r.Judge.Acceptable
		if p {
			pos++
		} else {
			neg++
		}
		items = append(items, scored{score(r), p})
	}
	if pos == 0 || neg == 0 {
		return 0, false
	}
	sort.Slice(items, func(i, j int) bool { return items[i].s < items[j].s })

	// Mann-Whitney U with midranks for ties.
	ranks := make([]float64, len(items))
	for i := 0; i < len(items); {
		j := i
		for j+1 < len(items) && items[j+1].s == items[i].s {
			j++
		}
		mid := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			ranks[k] = mid
		}
		i = j + 1
	}
	sumPos := 0.0
	for i, it := range items {
		if it.pos {
			sumPos += ranks[i]
		}
	}
	u := sumPos - float64(pos)*float64(pos+1)/2
	return u / (float64(pos) * float64(neg)), true
}

type AUROCResult struct {
	Name      string
	Point     float64
	Lower     float64
	Upper     float64
	Positives int
	Negatives int
	OK        bool
}

// BootstrapAUROC resamples records with replacement to put a 95% interval
// on AUROC. Resamples without both classes are skipped.
func BootstrapAUROC(records []*collect.Record, name string, score ScoreFunc, iterations int, seed int64) AUROCResult {
	res := AUROCResult{Name: name}
	point, ok := AUROC(records, score)
	if !ok {
		return res
	}
	res.OK = true
	res.Point = point
	for _, r := range records {
		if r.Error != "" {
			continue
		}
		if r.Judge.Acceptable {
			res.Negatives++
		} else {
			res.Positives++
		}
	}

	rng := rand.New(rand.NewSource(seed))
	n := len(records)
	sample := make([]*collect.Record, n)
	vals := make([]float64, 0, iterations)
	for range iterations {
		for j := range n {
			sample[j] = records[rng.Intn(n)]
		}
		if v, ok := AUROC(sample, score); ok {
			vals = append(vals, v)
		}
	}
	sort.Float64s(vals)
	res.Lower = percentile(vals, 2.5)
	res.Upper = percentile(vals, 97.5)
	return res
}
