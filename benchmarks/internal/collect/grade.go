package collect

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	GradingJudge   = "judge"
	GradingNumeric = "numeric"
	GradingChoice  = "choice"
)

var (
	finalAnswerRe = regexp.MustCompile(`(?i)final answer\s*[:=]?\s*\**\s*\$?\s*(-?[\d,]*\.?\d+)`)
	numberRe      = regexp.MustCompile(`-?\d[\d,]*\.?\d*`)
	choiceLineRe  = regexp.MustCompile(`(?i)answer\s*[:=]?\s*\**\s*\(?([A-E])\b`)
	leadChoiceRe  = regexp.MustCompile(`^\s*\(?([A-E])[\).:\s]`)
)

// ExtractNumericAnswer pulls the final numeric answer out of a free-form
// response. It prefers an explicit "Final answer: N" line and falls back
// to the last number in the text, which is how GSM8K is normally scored.
func ExtractNumericAnswer(response string) string {
	if m := finalAnswerRe.FindStringSubmatch(response); m != nil {
		return normalizeNumber(m[1])
	}
	nums := numberRe.FindAllString(response, -1)
	if len(nums) == 0 {
		return ""
	}
	return normalizeNumber(nums[len(nums)-1])
}

func normalizeNumber(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSuffix(s, ".")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ExtractChoiceAnswer pulls a single letter A-E out of a multiple-choice
// response, preferring the last explicit "Answer: X" line.
func ExtractChoiceAnswer(response string) string {
	matches := choiceLineRe.FindAllStringSubmatch(response, -1)
	if len(matches) > 0 {
		return strings.ToUpper(matches[len(matches)-1][1])
	}
	if m := leadChoiceRe.FindStringSubmatch(response); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

// Grade compares a response against ground truth under the given grading
// mode and returns the extracted answer plus whether it matched.
func Grade(grading, response, groundTruth string) (string, bool) {
	switch grading {
	case GradingNumeric:
		got := ExtractNumericAnswer(response)
		return got, got != "" && got == normalizeNumber(groundTruth)
	case GradingChoice:
		got := ExtractChoiceAnswer(response)
		return got, got != "" && got == strings.ToUpper(strings.TrimSpace(groundTruth))
	}
	return "", false
}
