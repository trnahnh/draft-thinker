package dataset

import (
	"encoding/json"
	"fmt"
	"os"
)

type Category string

const (
	SimpleFactual      Category = "simple_factual"
	MultiStepReasoning Category = "multi_step_reasoning"
	CodeGeneration     Category = "code_generation"
	AmbiguousCreative  Category = "ambiguous_creative"
)

var validCategories = map[Category]bool{
	SimpleFactual:      true,
	MultiStepReasoning: true,
	CodeGeneration:     true,
	AmbiguousCreative:  true,
}

type Prompt struct {
	ID       string   `json:"id"`
	Category Category `json:"category"`
	Text     string   `json:"text"`
}

type Dataset struct {
	Prompts []Prompt `json:"prompts"`
}

func Load(path string) (*Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading dataset: %w", err)
	}

	var ds Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parsing dataset: %w", err)
	}

	if len(ds.Prompts) == 0 {
		return nil, fmt.Errorf("dataset contains no prompts")
	}

	seen := make(map[string]bool)
	for i, p := range ds.Prompts {
		if p.ID == "" {
			return nil, fmt.Errorf("prompt at index %d has empty ID", i)
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("duplicate prompt ID: %s", p.ID)
		}
		seen[p.ID] = true

		if !validCategories[p.Category] {
			return nil, fmt.Errorf("prompt %s has invalid category: %q", p.ID, p.Category)
		}
		if p.Text == "" {
			return nil, fmt.Errorf("prompt %s has empty text", p.ID)
		}
	}

	return &ds, nil
}
