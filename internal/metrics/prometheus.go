package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusRecorder struct {
	registry             *prometheus.Registry
	requestsTotal        *prometheus.CounterVec
	upstreamLatency      *prometheus.HistogramVec
	errorsTotal          *prometheus.CounterVec
	entropyDistribution  prometheus.Histogram
	routingDecisionsTotal *prometheus.CounterVec
}

func NewPrometheusRecorder() *PrometheusRecorder {
	reg := prometheus.NewRegistry()

	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "draftthinker",
		Name:      "requests_total",
		Help:      "Total number of requests by model and HTTP status.",
	}, []string{"model", "status"})

	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "draftthinker",
		Name:      "upstream_latency_seconds",
		Help:      "Upstream LLM provider latency in seconds.",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"provider"})

	errors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "draftthinker",
		Name:      "errors_total",
		Help:      "Total number of errors by type.",
	}, []string{"type"})

	entropyDist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "draftthinker",
		Name:      "entropy_distribution",
		Help:      "Distribution of per-token Shannon entropy values in bits.",
		Buckets:   []float64{0, 0.25, 0.5, 0.75, 1.0, 1.5, 2.0, 2.5, 3.0},
	})

	routingDecisions := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "draftthinker",
		Name:      "routing_decisions_total",
		Help:      "Total routing decisions by outcome.",
	}, []string{"decision"})

	reg.MustRegister(requests, latency, errors, entropyDist, routingDecisions)

	return &PrometheusRecorder{
		registry:              reg,
		requestsTotal:         requests,
		upstreamLatency:       latency,
		errorsTotal:           errors,
		entropyDistribution:   entropyDist,
		routingDecisionsTotal: routingDecisions,
	}
}

func (p *PrometheusRecorder) RecordRequest(model string, status int) {
	p.requestsTotal.WithLabelValues(model, strconv.Itoa(status)).Inc()
}

func (p *PrometheusRecorder) RecordUpstreamLatency(provider string, d time.Duration) {
	p.upstreamLatency.WithLabelValues(provider).Observe(d.Seconds())
}

func (p *PrometheusRecorder) RecordError(errorType string) {
	p.errorsTotal.WithLabelValues(errorType).Inc()
}

func (p *PrometheusRecorder) RecordEntropy(value float64) {
	p.entropyDistribution.Observe(value)
}

func (p *PrometheusRecorder) RecordRoutingDecision(decision string) {
	p.routingDecisionsTotal.WithLabelValues(decision).Inc()
}

func (p *PrometheusRecorder) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}
