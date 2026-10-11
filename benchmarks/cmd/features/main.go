// features exports the learned-router feature vector for every valid
// record in one or more collected traces, as training data for
// benchmarks/train_router.py. The same Go code computes the features at
// inference time, so there is no feature parity problem between training
// and replay.
//
//	go run ./benchmarks/cmd/features -input benchmarks/results/collected_gsm8k.jsonl.gz,benchmarks/results/collected_mmlu.jsonl.gz -output benchmarks/results/features_train.csv
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/benchmarks/internal/sweep"
)

func main() {
	inputs := flag.String("input", "", "comma-separated collected JSONL (optionally .gz) traces")
	output := flag.String("output", "", "features CSV output path")
	prefix := flag.Int("prefix-tokens", 0, "restrict trace statistics to the first k drafter tokens (0 = whole draft)")
	flag.Parse()
	if *inputs == "" || *output == "" {
		log.Fatal("-input and -output are required")
	}

	f, err := os.Create(*output)
	if err != nil {
		log.Fatalf("creating output: %v", err)
	}
	defer f.Close()

	cfg := sweep.FeatureConfig{PrefixTokens: *prefix, Replay: sweep.ReplayConfig{WindowSize: 10, EarlyExitCount: 10}}
	total := 0
	first := true
	for _, in := range strings.Split(*inputs, ",") {
		in = strings.TrimSpace(in)
		if in == "" {
			continue
		}
		records, err := collect.LoadRecords(in)
		if err != nil {
			log.Fatalf("loading %s: %v", in, err)
		}
		valid := collect.ValidRecords(records)
		source := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(in), ".gz"), ".jsonl")
		if first {
			if err := sweep.WriteFeaturesCSV(f, valid, source, cfg); err != nil {
				log.Fatalf("writing features: %v", err)
			}
			first = false
		} else {
			if err := sweep.AppendFeaturesCSV(f, valid, source, cfg); err != nil {
				log.Fatalf("writing features: %v", err)
			}
		}
		total += len(valid)
		log.Printf("%s: %d valid records", in, len(valid))
	}
	log.Printf("wrote %d rows to %s (prefix_tokens=%d)", total, *output, *prefix)
}
