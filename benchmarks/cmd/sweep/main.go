package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/benchmarks/internal/sweep"
)

func main() {
	input := flag.String("input", "benchmarks/results/collected.jsonl", "collected JSONL input path")
	output := flag.String("output", "benchmarks/results/sweep.csv", "sweep CSV output path")
	thresholdsStr := flag.String("thresholds", "0.5,0.75,1.0,1.25,1.5", "comma-separated threshold values")
	windowSize := flag.Int("window-size", 10, "entropy window size")
	earlyExitCount := flag.Int("early-exit-count", 10, "early exit token count")
	flag.Parse()

	thresholds, err := parseThresholds(*thresholdsStr)
	if err != nil {
		log.Fatalf("parsing thresholds: %v", err)
	}

	records, err := loadRecords(*input)
	if err != nil {
		log.Fatalf("loading records: %v", err)
	}
	log.Printf("loaded %d records (%d with errors filtered out)", len(records), countErrors(records))

	var valid []*collect.Record
	for _, r := range records {
		if r.Error == "" {
			valid = append(valid, r)
		}
	}
	log.Printf("analyzing %d valid records across %d thresholds", len(valid), len(thresholds))

	replayCfg := sweep.ReplayConfig{
		WindowSize:     *windowSize,
		EarlyExitCount: *earlyExitCount,
	}
	pricing := sweep.DefaultPricing()

	var allStats []sweep.ThresholdStats
	for _, t := range thresholds {
		stats := sweep.ComputeStats(valid, t, replayCfg, pricing)
		allStats = append(allStats, stats)
	}

	csvFile, err := os.Create(*output)
	if err != nil {
		log.Fatalf("creating CSV output: %v", err)
	}
	defer csvFile.Close()

	if err := sweep.WriteCSV(csvFile, allStats); err != nil {
		log.Fatalf("writing CSV: %v", err)
	}
	log.Printf("wrote CSV to %s", *output)

	fmt.Println()
	if err := sweep.WriteSummary(os.Stdout, allStats); err != nil {
		log.Fatalf("writing summary: %v", err)
	}
}

func parseThresholds(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	thresholds := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid threshold %q: %w", p, err)
		}
		if v <= 0 {
			return nil, fmt.Errorf("threshold must be positive, got %f", v)
		}
		thresholds = append(thresholds, v)
	}
	if len(thresholds) == 0 {
		return nil, fmt.Errorf("no thresholds specified")
	}
	return thresholds, nil
}

func loadRecords(path string) ([]*collect.Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening records: %w", err)
	}
	defer f.Close()

	var records []*collect.Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r collect.Record
		if err := json.Unmarshal(line, &r); err != nil {
			log.Printf("warning: skipping malformed line %d: %v", lineNum, err)
			continue
		}
		records = append(records, &r)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading records: %w", err)
	}

	return records, nil
}

func countErrors(records []*collect.Record) int {
	n := 0
	for _, r := range records {
		if r.Error != "" {
			n++
		}
	}
	return n
}
