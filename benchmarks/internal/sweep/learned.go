package sweep

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

const MethodLearnedRouter Method = "learned_router"

// LearnedModel is a classifier fitted in Python (benchmarks/train_router.py)
// and exported as JSON so the Go evaluator can replay it over traces with
// no Python at inference time. Kind is "logistic" (standardised features,
// linear coefficients) or "gbt" (sklearn gradient-boosted trees). The
// optional isotonic calibration maps the raw probability to an empirical
// probability of a wrong draft, and Thresholds carry the operating points
// chosen on the calibration split.
type LearnedModel struct {
	Name         string           `json:"name"`
	Kind         string           `json:"kind"`
	Features     []string         `json:"features"`
	PrefixTokens int              `json:"prefix_tokens"`
	Mean         []float64        `json:"mean,omitempty"`
	Scale        []float64        `json:"scale,omitempty"`
	Coef         []float64        `json:"coef,omitempty"`
	Intercept    float64          `json:"intercept,omitempty"`
	InitScore    float64          `json:"init_score,omitempty"`
	LearningRate float64          `json:"learning_rate,omitempty"`
	Trees        []Tree           `json:"trees,omitempty"`
	Calibration  *Isotonic        `json:"calibration,omitempty"`
	Thresholds   []OperatingPoint `json:"thresholds,omitempty"`
	Train        map[string]any   `json:"train,omitempty"`
}

// Tree is an sklearn decision tree in array form. Node i is a leaf when
// Left[i] < 0; otherwise samples with x[Feature[i]] <= Threshold[i] go left.
type Tree struct {
	Feature   []int     `json:"feature"`
	Threshold []float64 `json:"threshold"`
	Left      []int     `json:"left"`
	Right     []int     `json:"right"`
	Value     []float64 `json:"value"`
}

// Isotonic is a monotone piecewise-linear map (sklearn IsotonicRegression
// with out_of_bounds="clip").
type Isotonic struct {
	X []float64 `json:"x"`
	Y []float64 `json:"y"`
}

// OperatingPoint is a probability threshold chosen on the calibration
// split for a named target, e.g. "served_error_le_0.05".
type OperatingPoint struct {
	Name        string  `json:"name"`
	Tau         float64 `json:"tau"`
	Target      float64 `json:"target,omitempty"`
	Description string  `json:"description,omitempty"`
}

func LoadLearnedModel(path string) (*LearnedModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading model: %w", err)
	}
	var m LearnedModel
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing model: %w", err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *LearnedModel) validate() error {
	if len(m.Features) != len(FeatureNames) {
		return fmt.Errorf("model has %d features, evaluator has %d", len(m.Features), len(FeatureNames))
	}
	for i, f := range m.Features {
		if f != FeatureNames[i] {
			return fmt.Errorf("feature %d is %q in model but %q in evaluator", i, f, FeatureNames[i])
		}
	}
	switch m.Kind {
	case "logistic":
		if len(m.Coef) != len(m.Features) || len(m.Mean) != len(m.Features) || len(m.Scale) != len(m.Features) {
			return fmt.Errorf("logistic model arrays do not match feature count")
		}
	case "gbt":
		if len(m.Trees) == 0 {
			return fmt.Errorf("gbt model has no trees")
		}
	default:
		return fmt.Errorf("unknown model kind %q", m.Kind)
	}
	if m.Calibration != nil && len(m.Calibration.X) != len(m.Calibration.Y) {
		return fmt.Errorf("calibration x and y differ in length")
	}
	return nil
}

func (m *LearnedModel) featureConfig() FeatureConfig {
	return FeatureConfig{PrefixTokens: m.PrefixTokens, Replay: ReplayConfig{WindowSize: 10, EarlyExitCount: 10}}
}

// RawProbability evaluates the classifier on a feature vector without
// calibration.
func (m *LearnedModel) RawProbability(x []float64) float64 {
	switch m.Kind {
	case "logistic":
		z := m.Intercept
		for i, c := range m.Coef {
			s := m.Scale[i]
			if s == 0 {
				s = 1
			}
			z += c * (x[i] - m.Mean[i]) / s
		}
		return sigmoid(z)
	case "gbt":
		z := m.InitScore
		for _, t := range m.Trees {
			z += m.LearningRate * t.predict(x)
		}
		return sigmoid(z)
	}
	return 0
}

// Probability is the calibrated probability that the draft is wrong, or
// the raw probability when the model carries no calibration map.
func (m *LearnedModel) Probability(x []float64) float64 {
	p := m.RawProbability(x)
	if m.Calibration != nil {
		return m.Calibration.Apply(p)
	}
	return p
}

// Score returns the raw (uncalibrated) probability for a record. Higher
// means more likely to need escalation, matching the other ScoreFuncs. The
// raw score is used for AUROC and rate matching because isotonic
// calibration is a step function whose ties make rate matching coarse.
func (m *LearnedModel) Score(r *collect.Record) float64 {
	return m.RawProbability(ExtractFeatures(r, m.featureConfig()))
}

// CalibratedScore is the calibrated probability that the draft is wrong,
// the quantity the exported operating points threshold.
func (m *LearnedModel) CalibratedScore(r *collect.Record) float64 {
	return m.Probability(ExtractFeatures(r, m.featureConfig()))
}

// Policy escalates when the raw probability exceeds tau. A prefix model
// decides after PrefixTokens drafter tokens, so TokensConsumed reflects
// the early exit; a full-trace model consumes the whole draft.
func (m *LearnedModel) Policy(tau float64) EscalationPolicy {
	return m.policy(m.Score, tau)
}

// CalibratedPolicy escalates when the calibrated probability exceeds tau,
// for replaying the operating points chosen on the calibration split.
func (m *LearnedModel) CalibratedPolicy(tau float64) EscalationPolicy {
	return m.policy(m.CalibratedScore, tau)
}

func (m *LearnedModel) policy(score ScoreFunc, tau float64) EscalationPolicy {
	return func(r *collect.Record) Decision {
		consumed := len(r.Tokens)
		if m.PrefixTokens > 0 && m.PrefixTokens < consumed {
			consumed = m.PrefixTokens
		}
		return Decision{Escalate: score(r) > tau, TokensConsumed: consumed}
	}
}

func (t Tree) predict(x []float64) float64 {
	node := 0
	for t.Left[node] >= 0 {
		if x[t.Feature[node]] <= t.Threshold[node] {
			node = t.Left[node]
		} else {
			node = t.Right[node]
		}
	}
	return t.Value[node]
}

// Apply interpolates linearly between breakpoints and clips outside them.
func (iso *Isotonic) Apply(p float64) float64 {
	n := len(iso.X)
	if n == 0 {
		return p
	}
	if p <= iso.X[0] {
		return iso.Y[0]
	}
	if p >= iso.X[n-1] {
		return iso.Y[n-1]
	}
	i := sort.SearchFloat64s(iso.X, p)
	x0, x1 := iso.X[i-1], iso.X[i]
	y0, y1 := iso.Y[i-1], iso.Y[i]
	if x1 == x0 {
		return y1
	}
	return y0 + (y1-y0)*(p-x0)/(x1-x0)
}

func sigmoid(z float64) float64 {
	return 1 / (1 + math.Exp(-z))
}
