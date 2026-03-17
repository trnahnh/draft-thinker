package dataset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")

	valid := `{
		"prompts": [
			{"id": "simple_factual_001", "category": "simple_factual", "text": "What is 2+2?"},
			{"id": "code_generation_001", "category": "code_generation", "text": "Write a hello world in Go"}
		]
	}`
	if err := os.WriteFile(path, []byte(valid), 0644); err != nil {
		t.Fatal(err)
	}

	ds, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ds.Prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(ds.Prompts))
	}
	if ds.Prompts[0].ID != "simple_factual_001" {
		t.Errorf("expected ID simple_factual_001, got %s", ds.Prompts[0].ID)
	}
	if ds.Prompts[1].Category != CodeGeneration {
		t.Errorf("expected category code_generation, got %s", ds.Prompts[1].Category)
	}
}

func TestLoadEmptyPrompts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	if err := os.WriteFile(path, []byte(`{"prompts": []}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty prompts")
	}
}

func TestLoadMissingID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	data := `{"prompts": [{"id": "", "category": "simple_factual", "text": "hello"}]}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestLoadInvalidCategory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	data := `{"prompts": [{"id": "p1", "category": "invalid_cat", "text": "hello"}]}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid category")
	}
}

func TestLoadEmptyText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	data := `{"prompts": [{"id": "p1", "category": "simple_factual", "text": ""}]}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestLoadDuplicateID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	data := `{"prompts": [
		{"id": "p1", "category": "simple_factual", "text": "hello"},
		{"id": "p1", "category": "simple_factual", "text": "world"}
	]}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadRealPrompts(t *testing.T) {
	path := "../../testdata/prompts.json"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("prompts.json not found, skipping")
	}

	ds, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load real prompts: %v", err)
	}

	if len(ds.Prompts) < 500 {
		t.Errorf("expected at least 500 prompts, got %d", len(ds.Prompts))
	}

	cats := make(map[Category]int)
	for _, p := range ds.Prompts {
		cats[p.Category]++
	}

	expected := map[Category]int{
		SimpleFactual:      150,
		MultiStepReasoning: 150,
		CodeGeneration:     110,
		AmbiguousCreative:  110,
	}
	for cat, want := range expected {
		if got := cats[cat]; got < want {
			t.Errorf("category %s: expected at least %d, got %d", cat, want, got)
		}
	}
}

func TestLoadAllCategories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	data := `{"prompts": [
		{"id": "a", "category": "simple_factual", "text": "q1"},
		{"id": "b", "category": "multi_step_reasoning", "text": "q2"},
		{"id": "c", "category": "code_generation", "text": "q3"},
		{"id": "d", "category": "ambiguous_creative", "text": "q4"}
	]}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	ds, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ds.Prompts) != 4 {
		t.Fatalf("expected 4 prompts, got %d", len(ds.Prompts))
	}
}
