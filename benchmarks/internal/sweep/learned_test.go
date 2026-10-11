package sweep

import (
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

// TestLearnedModelParity checks the Go evaluator against probabilities the
// Python trainer computed for the same feature vectors, for every exported
// model that has a parity file.
func TestLearnedModelParity(t *testing.T) {
	cases := []struct{ model, parity string }{
		{"../../models/router_full_logistic.json", "../../models/router_full_parity.csv"},
		{"../../models/router_full_gbt.json", "../../models/router_full_parity.csv"},
	}
	for _, c := range cases {
		m, err := LoadLearnedModel(c.model)
		if err != nil {
			t.Fatalf("loading %s: %v", c.model, err)
		}
		rows := readParity(t, c.parity, m.Name)
		if len(rows) == 0 {
			t.Fatalf("%s: no parity rows for model %s", c.parity, m.Name)
		}
		for _, row := range rows {
			raw := m.RawProbability(row.x)
			if math.Abs(raw-row.raw) > 1e-9 {
				t.Errorf("%s %s: raw %.12f, python %.12f", m.Name, row.id, raw, row.raw)
			}
			cal := m.Probability(row.x)
			if math.Abs(cal-row.cal) > 1e-9 {
				t.Errorf("%s %s: calibrated %.12f, python %.12f", m.Name, row.id, cal, row.cal)
			}
		}
	}
}

type parityRow struct {
	id       string
	x        []float64
	raw, cal float64
}

func readParity(t *testing.T, path, model string) []parityRow {
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	header := records[0]
	nf := len(FeatureNames)
	if len(header) != 2+nf+2 {
		t.Fatalf("parity header has %d columns, want %d", len(header), 2+nf+2)
	}
	for i, name := range FeatureNames {
		if header[2+i] != name {
			t.Fatalf("parity feature %d is %q, evaluator has %q", i, header[2+i], name)
		}
	}
	var out []parityRow
	for _, rec := range records[1:] {
		if rec[0] != model {
			continue
		}
		row := parityRow{id: rec[1], x: make([]float64, nf)}
		for i := range nf {
			row.x[i] = parseF(t, rec[2+i])
		}
		row.raw = parseF(t, rec[2+nf])
		row.cal = parseF(t, rec[2+nf+1])
		out = append(out, row)
	}
	return out
}

func parseF(t *testing.T, s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("parsing %q: %v", s, err)
	}
	return v
}

func TestIsotonicApply(t *testing.T) {
	iso := &Isotonic{X: []float64{0.1, 0.5, 0.9}, Y: []float64{0.0, 0.2, 1.0}}
	cases := map[float64]float64{0.0: 0.0, 0.1: 0.0, 0.3: 0.1, 0.5: 0.2, 0.7: 0.6, 0.9: 1.0, 1.0: 1.0}
	for in, want := range cases {
		if got := iso.Apply(in); math.Abs(got-want) > 1e-12 {
			t.Errorf("Apply(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestTreePredict(t *testing.T) {
	tree := Tree{
		Feature:   []int{0, -2, 1, -2, -2},
		Threshold: []float64{0.5, 0, 2.0, 0, 0},
		Left:      []int{1, -1, 3, -1, -1},
		Right:     []int{2, -1, 4, -1, -1},
		Value:     []float64{0, -1, 0, 0.25, 0.75},
	}
	if got := tree.predict([]float64{0.2, 9}); got != -1 {
		t.Errorf("left leaf = %v", got)
	}
	if got := tree.predict([]float64{0.9, 1}); got != 0.25 {
		t.Errorf("right-left leaf = %v", got)
	}
	if got := tree.predict([]float64{0.9, 3}); got != 0.75 {
		t.Errorf("right-right leaf = %v", got)
	}
}

func TestExtractFeaturesPrefix(t *testing.T) {
	r := &collect.Record{Category: "gsm8k", DraftResponse: "x"}
	for i := range 30 {
		r.Tokens = append(r.Tokens, collect.TokenRecord{Token: "t", Entropy: 0.2, Logprob: -0.1, Index: i})
	}
	r.TokenCount = 30
	r.Tokens[25].Entropy = 2.3
	full := ExtractFeatures(r, FeatureConfig{})
	prefix := ExtractFeatures(r, FeatureConfig{PrefixTokens: 20})
	if full[1] != 2.3 {
		t.Errorf("full max entropy = %v, want 2.3", full[1])
	}
	if prefix[1] != 0.2 {
		t.Errorf("prefix max entropy = %v, want 0.2 (token 25 is outside the prefix)", prefix[1])
	}
	if prefix[8] != 20 {
		t.Errorf("prefix token_count = %v, want 20", prefix[8])
	}
	if len(full) != len(FeatureNames) {
		t.Errorf("feature vector length %d, names %d", len(full), len(FeatureNames))
	}
}
