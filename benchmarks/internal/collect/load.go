package collect

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// LoadRecords reads a JSONL trace file, transparently decompressing
// files that end in .gz. Malformed lines are skipped with a warning so
// a partially written line from an interrupted collection run does not
// invalidate the rest of the dataset.
func LoadRecords(path string) ([]*Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening records: %w", err)
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("opening gzip: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	var records []*Record
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
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

// ValidRecords filters out records whose collection failed.
func ValidRecords(records []*Record) []*Record {
	var valid []*Record
	for _, r := range records {
		if r.Error == "" {
			valid = append(valid, r)
		}
	}
	return valid
}
