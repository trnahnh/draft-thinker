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
	GSM8K              Category = "gsm8k"
	MMLU               Category = "mmlu"
)

var validCategories = map[Category]bool{
	SimpleFactual:      true,
	MultiStepReasoning: true,
	CodeGeneration:     true,
	AmbiguousCreative:  true,
	GSM8K:              true,
	MMLU:               true,
}

var validGradings = map[string]bool{
	"":        true,
	"judge":   true,
	"numeric": true,
	"choice":  true,
}

type Prompt struct {
	ID       string   `json:"id"`
	Category Category `json:"category"`
	Text     string   `json:"text"`
	// Answer and Grading are set for public benchmarks with an answer key.
	// Grading is "numeric" or "choice"; empty means LLM-as-judge.
	Answer  string `json:"answer,omitempty"`
	Grading string `json:"grading,omitempty"`
	// Subject is optional provenance metadata (e.g. the MMLU subject).
	Subject string `json:"subject,omitempty"`
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
		if !validGradings[p.Grading] {
			return nil, fmt.Errorf("prompt %s has invalid grading: %q", p.ID, p.Grading)
		}
		if p.Grading != "" && p.Grading != "judge" && p.Answer == "" {
			return nil, fmt.Errorf("prompt %s uses %s grading but has no answer", p.ID, p.Grading)
		}
	}

	return &ds, nil
}

// Save writes the dataset as indented JSON.
func Save(path string, ds *Dataset) error {
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding dataset: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
