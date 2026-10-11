package collect

import "testing"

func TestExtractNumericAnswer(t *testing.T) {
	cases := map[string]string{
		"She makes 9 * 2 = $18 every day.\nFinal answer: 18":           "18",
		"Final Answer: $1,250.00":                                      "1250",
		"The total is 42.\nSo the answer is 42":                        "42",
		"Final answer: -3.5":                                           "-3.5",
		"No numbers here":                                              "",
		"Steps: 16 - 3 - 4 = 9 eggs. 9 * 2 = 18. **Final answer: 18**": "18",
	}
	for in, want := range cases {
		if got := ExtractNumericAnswer(in); got != want {
			t.Errorf("ExtractNumericAnswer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractChoiceAnswer(t *testing.T) {
	cases := map[string]string{
		"Reasoning... Answer: C":              "C",
		"Answer: (B)":                         "B",
		"The correct choice is D.\nAnswer: D": "D",
		"B. Because of the second law":        "B",
		"I am not sure":                       "",
		"**Answer: A**":                       "A",
		"Answer: E":                           "E",
		"Answer: F":                           "",
	}
	for in, want := range cases {
		if got := ExtractChoiceAnswer(in); got != want {
			t.Errorf("ExtractChoiceAnswer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGrade(t *testing.T) {
	if _, ok := Grade(GradingNumeric, "Final answer: 1,000", "1000"); !ok {
		t.Error("numeric grade should match 1,000 against 1000")
	}
	if _, ok := Grade(GradingNumeric, "Final answer: 7", "8"); ok {
		t.Error("numeric grade should not match 7 against 8")
	}
	if _, ok := Grade(GradingChoice, "Answer: b", "B"); !ok {
		t.Error("choice grade should be case-insensitive")
	}
	if _, ok := Grade(GradingJudge, "anything", "x"); ok {
		t.Error("judge grading has no exact-match semantics")
	}
}

func TestEstimatedPromptTokens(t *testing.T) {
	r := &Record{PromptText: "What is 347 + 892?"}
	if got := r.EstimatedPromptTokens(); got != 5 {
		t.Errorf("estimate = %d, want 5", got)
	}
	r.PromptTokens = 12
	if got := r.EstimatedPromptTokens(); got != 12 {
		t.Errorf("reported tokens should win, got %d", got)
	}
}
