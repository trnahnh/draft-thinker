package collect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Store struct {
	path      string
	collected map[string]bool
	mu        sync.Mutex
}

func NewStore(path string) (*Store, error) {
	s := &Store{
		path:      path,
		collected: make(map[string]bool),
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("opening store: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var partial struct {
			PromptID string `json:"prompt_id"`
		}
		if err := json.Unmarshal(line, &partial); err != nil {
			continue
		}
		if partial.PromptID != "" {
			s.collected[partial.PromptID] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning store: %w", err)
	}

	return s, nil
}

func (s *Store) AlreadyCollected(promptID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collected[promptID]
}

func (s *Store) Append(record *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling record: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening store for append: %w", err)
	}
	defer f.Close()

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing record: %w", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing store: %w", err)
	}

	s.collected[record.PromptID] = true
	return nil
}

func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.collected)
}
