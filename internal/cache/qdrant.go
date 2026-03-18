package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type qdrantClient struct {
	baseURL    string
	collection string
	httpClient *http.Client
}

func newQdrantClient(baseURL, collection string, timeout time.Duration) *qdrantClient {
	return &qdrantClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		collection: collection,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (q *qdrantClient) EnsureCollection(ctx context.Context, dimensions int) error {
	body := map[string]any{
		"vectors": map[string]any{
			"size":     dimensions,
			"distance": "Cosine",
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling collection config: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", q.baseURL, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating collection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("creating collection: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("qdrant create collection returned %d", resp.StatusCode)
	}

	return nil
}

type qdrantSearchRequest struct {
	Vector         []float64 `json:"vector"`
	Limit          int       `json:"limit"`
	ScoreThreshold float64   `json:"score_threshold"`
	WithPayload    bool      `json:"with_payload"`
}

type qdrantSearchResponse struct {
	Result []qdrantSearchHit `json:"result"`
}

type qdrantSearchHit struct {
	ID      string            `json:"id"`
	Score   float64           `json:"score"`
	Payload map[string]string `json:"payload"`
}

func (q *qdrantClient) Search(ctx context.Context, vector []float64, threshold float64) (*SearchResult, error) {
	body := qdrantSearchRequest{
		Vector:         vector,
		Limit:          1,
		ScoreThreshold: threshold,
		WithPayload:    true,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling search request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", q.baseURL, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qdrant search returned %d: %s", resp.StatusCode, respBody)
	}

	var result qdrantSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}

	if len(result.Result) == 0 {
		return nil, nil
	}

	hit := result.Result[0]
	return &SearchResult{
		ID:      hit.ID,
		Score:   hit.Score,
		Payload: hit.Payload,
	}, nil
}

type qdrantUpsertRequest struct {
	Points []qdrantPoint `json:"points"`
}

type qdrantPoint struct {
	ID      string            `json:"id"`
	Vector  []float64         `json:"vector"`
	Payload map[string]string `json:"payload"`
}

func (q *qdrantClient) Upsert(ctx context.Context, id string, vector []float64, payload map[string]string) error {
	body := qdrantUpsertRequest{
		Points: []qdrantPoint{
			{ID: id, Vector: vector, Payload: payload},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling upsert request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", q.baseURL, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant upsert: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant upsert returned %d", resp.StatusCode)
	}

	return nil
}

func (q *qdrantClient) Delete(ctx context.Context, ids []string) error {
	body := map[string]any{
		"points": ids,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling delete request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", q.baseURL, q.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant delete: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant delete returned %d", resp.StatusCode)
	}

	return nil
}
