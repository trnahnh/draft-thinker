package collect

import (
	"path/filepath"
	"testing"
)

func TestStoreAppendAndResume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if s.AlreadyCollected("p1") {
		t.Error("p1 should not be collected yet")
	}

	r := &Record{PromptID: "p1", Category: "simple_factual", PromptText: "hello"}
	if err := s.Append(r); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if !s.AlreadyCollected("p1") {
		t.Error("p1 should be collected after append")
	}
	if s.Count() != 1 {
		t.Errorf("expected count 1, got %d", s.Count())
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reopen NewStore: %v", err)
	}
	if !s2.AlreadyCollected("p1") {
		t.Error("p1 should be collected after reopen")
	}
	if s2.Count() != 1 {
		t.Errorf("expected count 1 after reopen, got %d", s2.Count())
	}

	r2 := &Record{PromptID: "p2", Category: "code_generation", PromptText: "world"}
	if err := s2.Append(r2); err != nil {
		t.Fatalf("second Append: %v", err)
	}
	if s2.Count() != 2 {
		t.Errorf("expected count 2, got %d", s2.Count())
	}
}

func TestStoreEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if s.Count() != 0 {
		t.Errorf("expected count 0, got %d", s.Count())
	}
}

func TestStoreMultipleRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.jsonl")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	for i := 0; i < 5; i++ {
		r := &Record{PromptID: "p" + string(rune('a'+i)), PromptText: "text"}
		if err := s.Append(r); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	if s.Count() != 5 {
		t.Errorf("expected 5, got %d", s.Count())
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if s2.Count() != 5 {
		t.Errorf("expected 5 after reopen, got %d", s2.Count())
	}
}
