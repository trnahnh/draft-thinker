// fetchpublic builds benchmark prompt files from public datasets with
// answer keys, so routing accuracy can be scored by exact match instead of
// an LLM judge. Supported: GSM8K test split (numeric answers), MMLU test
// split (four-way multiple choice) and ARC-Challenge test split (three- to
// five-way multiple choice). Sampling is seeded so the resulting prompt
// file is reproducible from the command line alone. Previously sampled
// prompt files can be excluded so a second sample is disjoint from the
// first.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/dataset"
)

const (
	gsm8kURL     = "https://raw.githubusercontent.com/openai/grade-school-math/master/grade_school_math/data/test.jsonl"
	mmluRowsURL  = "https://datasets-server.huggingface.co/rows?dataset=cais/mmlu&config=all&split=test"
	arcRowsURL   = "https://datasets-server.huggingface.co/rows?dataset=allenai/ai2_arc&config=ARC-Challenge&split=test"
	mmluTestRows = 14042
	arcTestRows  = 1172
	hfPacing     = 1500 * time.Millisecond
)

var letters = []string{"A", "B", "C", "D", "E"}

var testRowRe = regexp.MustCompile(`test_row_(\d+)`)

func main() {
	name := flag.String("dataset", "gsm8k", "gsm8k, mmlu or arc")
	n := flag.Int("n", 300, "number of prompts to sample")
	seed := flag.Int64("seed", 42, "sampling seed")
	output := flag.String("output", "", "output prompts JSON path (default benchmarks/testdata/<dataset>_<n>.json)")
	exclude := flag.String("exclude", "", "comma-separated prompt files whose source rows must not be sampled again")
	idPrefix := flag.String("id-prefix", "", "prompt ID prefix (default <dataset>_)")
	flag.Parse()

	if *output == "" {
		*output = fmt.Sprintf("benchmarks/testdata/%s_%d.json", *name, *n)
	}
	if *idPrefix == "" {
		*idPrefix = *name + "_"
	}

	excluded, err := loadExcludedRows(*exclude)
	if err != nil {
		log.Fatalf("loading exclusions: %v", err)
	}
	if len(excluded) > 0 {
		log.Printf("excluding %d previously sampled source rows", len(excluded))
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	rng := rand.New(rand.NewSource(*seed))

	var prompts []dataset.Prompt
	switch *name {
	case "gsm8k":
		prompts, err = fetchGSM8K(httpClient, rng, *n, *idPrefix, excluded)
	case "mmlu":
		prompts, err = fetchMMLU(httpClient, rng, *n, *idPrefix, excluded)
	case "arc":
		prompts, err = fetchARC(httpClient, rng, *n, *idPrefix, excluded)
	default:
		log.Fatalf("unknown dataset %q", *name)
	}
	if err != nil {
		log.Fatalf("fetching %s: %v", *name, err)
	}

	ds := &dataset.Dataset{Prompts: prompts}
	if err := dataset.Save(*output, ds); err != nil {
		log.Fatalf("saving: %v", err)
	}
	if _, err := dataset.Load(*output); err != nil {
		log.Fatalf("validating saved dataset: %v", err)
	}
	log.Printf("wrote %d prompts to %s", len(prompts), *output)
}

// loadExcludedRows reads prompt files written by this tool and returns the
// source row indices they were sampled from, parsed from the Subject field.
func loadExcludedRows(paths string) (map[int]bool, error) {
	out := map[int]bool{}
	for _, p := range strings.Split(paths, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		ds, err := dataset.Load(p)
		if err != nil {
			return nil, err
		}
		for _, pr := range ds.Prompts {
			m := testRowRe.FindStringSubmatch(pr.Subject)
			if m == nil {
				return nil, fmt.Errorf("%s: prompt %s has no test_row marker in subject %q", p, pr.ID, pr.Subject)
			}
			idx, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, err
			}
			out[idx] = true
		}
	}
	return out, nil
}

type gsm8kRow struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func fetchGSM8K(c *http.Client, rng *rand.Rand, n int, idPrefix string, excluded map[int]bool) ([]dataset.Prompt, error) {
	resp, err := c.Get(gsm8kURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var rows []gsm8kRow
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r gsm8kRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("parsing row: %w", err)
		}
		rows = append(rows, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	log.Printf("gsm8k test split: %d rows", len(rows))

	chosen := sampleIndices(rng, len(rows), n, excluded)

	prompts := make([]dataset.Prompt, 0, n)
	for i, rowIdx := range chosen {
		r := rows[rowIdx]
		sep := strings.LastIndex(r.Answer, "####")
		if sep < 0 {
			return nil, fmt.Errorf("row %d has no #### answer marker", rowIdx)
		}
		answer := strings.TrimSpace(r.Answer[sep+4:])
		answer = strings.ReplaceAll(answer, ",", "")
		prompts = append(prompts, dataset.Prompt{
			ID:       fmt.Sprintf("%s%04d", idPrefix, i+1),
			Category: dataset.GSM8K,
			Text:     r.Question + "\n\nSolve the problem step by step. On the last line, write only: Final answer: <number>",
			Answer:   answer,
			Grading:  "numeric",
			Subject:  fmt.Sprintf("test_row_%d", rowIdx),
		})
	}
	return prompts, nil
}

// sampleIndices draws n distinct indices from [0, total) in seeded random
// order, skipping excluded ones, and returns them sorted. With no
// exclusions it reproduces the original fetchpublic sampling exactly.
func sampleIndices(rng *rand.Rand, total, n int, excluded map[int]bool) []int {
	perm := rng.Perm(total)
	chosen := make([]int, 0, n)
	for _, idx := range perm {
		if len(chosen) >= n {
			break
		}
		if excluded[idx] {
			continue
		}
		chosen = append(chosen, idx)
	}
	sort.Ints(chosen)
	return chosen
}

type mmluRowsResponse struct {
	Rows []struct {
		RowIdx int `json:"row_idx"`
		Row    struct {
			Question string   `json:"question"`
			Subject  string   `json:"subject"`
			Choices  []string `json:"choices"`
			Answer   int      `json:"answer"`
		} `json:"row"`
	} `json:"rows"`
}

// fetchMMLU samples rows from the Hugging Face datasets-server rows API.
// The test split is ordered by subject, so it draws small pages (five rows)
// at many distinct random offsets rather than one large page, which would
// collapse the sample onto a handful of subjects. Requests are paced
// because the API rate-limits aggressively. Excluded rows are skipped and
// further pages are drawn until n prompts are collected.
func fetchMMLU(c *http.Client, rng *rand.Rand, n int, idPrefix string, excluded map[int]bool) ([]dataset.Prompt, error) {
	const pageLen = 5
	maxStart := mmluTestRows - pageLen
	perm := rng.Perm(maxStart)

	prompts := make([]dataset.Prompt, 0, n)
	pages := 0
	for _, off := range perm {
		if len(prompts) >= n {
			break
		}
		if pages > 0 {
			time.Sleep(hfPacing)
		}
		pages++
		url := fmt.Sprintf("%s&offset=%d&length=%d", mmluRowsURL, off, pageLen)
		var parsed mmluRowsResponse
		if err := getJSONWithRetry(c, url, &parsed); err != nil {
			return nil, fmt.Errorf("offset %d: %w", off, err)
		}
		if len(parsed.Rows) != pageLen {
			return nil, fmt.Errorf("offset %d: expected %d rows, got %d", off, pageLen, len(parsed.Rows))
		}
		for _, pr := range parsed.Rows {
			if len(prompts) >= n {
				break
			}
			if excluded[pr.RowIdx] {
				continue
			}
			row := pr.Row
			if len(row.Choices) != 4 || row.Answer < 0 || row.Answer > 3 {
				return nil, fmt.Errorf("row %d: unexpected choices/answer shape", pr.RowIdx)
			}
			prompts = append(prompts, dataset.Prompt{
				ID:       fmt.Sprintf("%s%04d", idPrefix, len(prompts)+1),
				Category: dataset.MMLU,
				Text:     choiceQuestion(row.Question, row.Choices),
				Answer:   letters[row.Answer],
				Grading:  "choice",
				Subject:  fmt.Sprintf("%s (test_row_%d)", row.Subject, pr.RowIdx),
			})
		}
		if pages%10 == 0 {
			log.Printf("mmlu: fetched %d pages, %d/%d prompts", pages, len(prompts), n)
		}
	}
	if len(prompts) < n {
		return nil, fmt.Errorf("only %d of %d prompts available after exclusions", len(prompts), n)
	}
	return prompts, nil
}

type arcRowsResponse struct {
	Rows []struct {
		RowIdx int `json:"row_idx"`
		Row    struct {
			ID       string `json:"id"`
			Question string `json:"question"`
			Choices  struct {
				Text  []string `json:"text"`
				Label []string `json:"label"`
			} `json:"choices"`
			AnswerKey string `json:"answerKey"`
		} `json:"row"`
	} `json:"rows"`
}

// fetchARC downloads the whole ARC-Challenge test split (1172 rows, twelve
// pages) and samples from it locally, so the draw is uniform over rows.
// Questions have three to five choices; labels are relettered A to E by
// position because the source mixes letter and digit labels.
func fetchARC(c *http.Client, rng *rand.Rand, n int, idPrefix string, excluded map[int]bool) ([]dataset.Prompt, error) {
	const pageLen = 100
	type arcRow struct {
		rowIdx   int
		question string
		choices  []string
		answer   string
		sourceID string
	}
	var rows []arcRow
	for off := 0; off < arcTestRows; off += pageLen {
		if off > 0 {
			time.Sleep(hfPacing)
		}
		url := fmt.Sprintf("%s&offset=%d&length=%d", arcRowsURL, off, pageLen)
		var parsed arcRowsResponse
		if err := getJSONWithRetry(c, url, &parsed); err != nil {
			return nil, fmt.Errorf("offset %d: %w", off, err)
		}
		for _, pr := range parsed.Rows {
			r := pr.Row
			if len(r.Choices.Text) != len(r.Choices.Label) || len(r.Choices.Text) < 2 || len(r.Choices.Text) > len(letters) {
				return nil, fmt.Errorf("row %d: unexpected choices shape", pr.RowIdx)
			}
			answerPos := -1
			for i, label := range r.Choices.Label {
				if label == r.AnswerKey {
					answerPos = i
				}
			}
			if answerPos < 0 {
				return nil, fmt.Errorf("row %d: answer key %q not among labels", pr.RowIdx, r.AnswerKey)
			}
			rows = append(rows, arcRow{rowIdx: pr.RowIdx, question: r.Question, choices: r.Choices.Text, answer: letters[answerPos], sourceID: r.ID})
		}
	}
	if len(rows) != arcTestRows {
		return nil, fmt.Errorf("expected %d rows, got %d", arcTestRows, len(rows))
	}
	log.Printf("arc-challenge test split: %d rows", len(rows))

	chosen := sampleIndices(rng, len(rows), n, excluded)
	prompts := make([]dataset.Prompt, 0, n)
	for i, idx := range chosen {
		r := rows[idx]
		prompts = append(prompts, dataset.Prompt{
			ID:       fmt.Sprintf("%s%04d", idPrefix, i+1),
			Category: dataset.ARC,
			Text:     choiceQuestion(r.question, r.choices),
			Answer:   r.answer,
			Grading:  "choice",
			Subject:  fmt.Sprintf("%s (test_row_%d)", r.sourceID, r.rowIdx),
		})
	}
	return prompts, nil
}

func choiceQuestion(question string, choices []string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(question))
	b.WriteString("\n\n")
	for j, choice := range choices {
		fmt.Fprintf(&b, "%s. %s\n", letters[j], strings.TrimSpace(choice))
	}
	b.WriteString("\nThink step by step briefly, then on the last line write only: Answer: <letter>")
	return b.String()
}

func getJSONWithRetry(c *http.Client, url string, out any) error {
	var lastErr error
	for attempt := range 6 {
		if attempt > 0 {
			time.Sleep(time.Duration(5*attempt*attempt) * time.Second)
		}
		resp, err := c.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 200)])))
			continue
		}
		return json.Unmarshal(body, out)
	}
	return lastErr
}
