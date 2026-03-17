package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrometheusRecorder_RecordRequest(t *testing.T) {
	rec := NewPrometheusRecorder()
	rec.RecordRequest("llama3-8b-8192", 200)
	rec.RecordRequest("llama3-8b-8192", 200)
	rec.RecordRequest("llama3-8b-8192", 500)

	body := scrapeMetrics(t, rec)

	if !strings.Contains(body, `draftthinker_requests_total{model="llama3-8b-8192",status="200"} 2`) {
		t.Errorf("expected 2 requests with status 200, got:\n%s", body)
	}
	if !strings.Contains(body, `draftthinker_requests_total{model="llama3-8b-8192",status="500"} 1`) {
		t.Errorf("expected 1 request with status 500, got:\n%s", body)
	}
}

func TestPrometheusRecorder_RecordUpstreamLatency(t *testing.T) {
	rec := NewPrometheusRecorder()
	rec.RecordUpstreamLatency("groq", 100*time.Millisecond)
	rec.RecordUpstreamLatency("groq", 200*time.Millisecond)

	body := scrapeMetrics(t, rec)

	if !strings.Contains(body, "draftthinker_upstream_latency_seconds") {
		t.Errorf("expected upstream latency metric, got:\n%s", body)
	}
	if !strings.Contains(body, `draftthinker_upstream_latency_seconds_count{provider="groq"} 2`) {
		t.Errorf("expected 2 observations, got:\n%s", body)
	}
}

func TestPrometheusRecorder_RecordError(t *testing.T) {
	rec := NewPrometheusRecorder()
	rec.RecordError("upstream_timeout")
	rec.RecordError("upstream_timeout")
	rec.RecordError("invalid_request")

	body := scrapeMetrics(t, rec)

	if !strings.Contains(body, `draftthinker_errors_total{type="upstream_timeout"} 2`) {
		t.Errorf("expected 2 timeout errors, got:\n%s", body)
	}
	if !strings.Contains(body, `draftthinker_errors_total{type="invalid_request"} 1`) {
		t.Errorf("expected 1 invalid_request error, got:\n%s", body)
	}
}

func scrapeMetrics(t *testing.T, rec *PrometheusRecorder) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("metrics endpoint returned %d", w.Code)
	}
	return w.Body.String()
}
