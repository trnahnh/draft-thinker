// fetchpublic builds benchmark prompt files from public datasets with
// answer keys, so routing accuracy can be scored by exact match instead of
// an LLM judge. Supported: GSM8K test split (numeric answers) and MMLU test
// split (four-way multiple choice). Sampling is seeded so the resulting
// prompt file is reproducible from the command line alone.
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
	"sort"
	"strings"
	"time"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/dataset"
)

const (
	gsm8kURL     = "https://raw.githubusercontent.com/openai/grade-school-math/master/grade_school_math/data/test.jsonl"
	mmluRowsURL  = "https://datasets-server.huggingface.co/rows?dataset=cais/mmlu&config=all&split=test"
	mmluTestRows = 14042
)

func main() {
	name := flag.String("dataset", "gsm8k", "gsm8k or mmlu")
	n := flag.Int("n", 300, "number of prompts to sample")
	seed := flag.Int64("seed", 42, "sampling seed")
	output := flag.String("output", "", "output prompts JSON path (default benchmarks/testdata/<dataset>_<n>.json)")
	flag.Parse()

	if *output == "" {
		*output = fmt.Sprintf("benchmarks/testdata/%s_%d.json", *name, *n)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	rng := rand.New(rand.NewSource(*seed))

	var prompts []dataset.Prompt
	var err error
	switch *name {
	case "gsm8k":
		prompts, err = fetchGSM8K(httpClient, rng, *n)
	case "mmlu":
		prompts, err = fetchMMLU(httpClient, rng, *n)
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

type gsm8kRow struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func fetchGSM8K(c *http.Client, rng *rand.Rand, n int) ([]dataset.Prompt, error) {
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

	idx := rng.Perm(len(rows))
	if n > len(idx) {
		n = len(idx)
	}
	chosen := idx[:n]
	sort.Ints(chosen)

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
			ID:       fmt.Sprintf("gsm8k_%04d", i+1),
			Category: dataset.GSM8K,
			Text:     r.Question + "\n\nSolve the problem step by step. On the last line, write only: Final answer: <number>",
			Answer:   answer,
			Grading:  "numeric",
			Subject:  fmt.Sprintf("test_row_%d", rowIdx),
		})
	}
	return prompts, nil
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
// The test split is ordered by subject, so it draws n distinct random row
// offsets one at a time rather than contiguous pages, which would collapse
// the sample onto a handful of subjects.
func fetchMMLU(c *http.Client, rng *rand.Rand, n int) ([]dataset.Prompt, error) {
	perm := rng.Perm(mmluTestRows)
	if n > len(perm) {
		n = len(perm)
	}
	offsets := perm[:n]
	sort.Ints(offsets)

	prompts := make([]dataset.Prompt, 0, n)
	for i, off := range offsets {
		url := fmt.Sprintf("%s&offset=%d&length=1", mmluRowsURL, off)
		var parsed mmluRowsResponse
		if err := getJSONWithRetry(c, url, &parsed); err != nil {
			return nil, fmt.Errorf("offset %d: %w", off, err)
		}
		if len(parsed.Rows) != 1 {
			return nil, fmt.Errorf("offset %d: expected 1 row, got %d", off, len(parsed.Rows))
		}
		row := parsed.Rows[0].Row
		if len(row.Choices) != 4 || row.Answer < 0 || row.Answer > 3 {
			return nil, fmt.Errorf("offset %d: unexpected choices/answer shape", off)
		}
		letters := []string{"A", "B", "C", "D"}
		var b strings.Builder
		b.WriteString(strings.TrimSpace(row.Question))
		b.WriteString("\n\n")
		for j, choice := range row.Choices {
			fmt.Fprintf(&b, "%s. %s\n", letters[j], strings.TrimSpace(choice))
		}
		b.WriteString("\nThink step by step briefly, then on the last line write only: Answer: <letter>")
		prompts = append(prompts, dataset.Prompt{
			ID:       fmt.Sprintf("mmlu_%04d", i+1),
			Category: dataset.MMLU,
			Text:     b.String(),
			Answer:   letters[row.Answer],
			Grading:  "choice",
			Subject:  row.Subject,
		})
		if (i+1)%50 == 0 {
			log.Printf("mmlu: fetched %d/%d", i+1, n)
		}
	}
	return prompts, nil
}

func getJSONWithRetry(c *http.Client, url string, out any) error {
	var lastErr error
	for attempt := range 4 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
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
