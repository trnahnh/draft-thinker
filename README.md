# Draft-Thinker

A cost-aware LLM gateway in Go that reduces inference costs by routing requests through entropy-based draft-and-verify logic. Cheap model drafts first, expensive model only when the drafter isn't confident.

## The Problem

LLM-powered applications send 100% of traffic to frontier models regardless of query complexity. "What are your hours?" costs the same as "Explain the tradeoffs between B-tree and LSM-tree storage engines." This is wasteful in three ways:

- **Cost**: 70%+ of queries are answerable by models costing 10–50x less.
- **Latency**: Frontier models have 2–5x higher time-to-first-token than small models.
- **Scale**: At high throughput, frontier model rate limits become the bottleneck — not your application.

The hard part isn't routing — it's knowing *when* the cheap model is good enough without already having the right answer. Prompt classifiers ("is this question easy?") fail on distribution shift. A syntactically simple question can require complex reasoning depending on context.

## The Approach

Draft-Thinker solves this by analyzing the drafter model's own confidence signals during generation. Every token a model produces comes with log-probabilities for its top candidates. High entropy (uncertainty) across those candidates means the model is guessing. Low entropy means it's confident.

The gateway watches these signals in real-time as the drafter generates. If confidence stays high, ship the draft. If it drops, escalate to the heavyweight.

### Entropy-Based Routing

The core mechanism computes Shannon entropy over the drafter's token logprobabilities using a sliding window. A calibrated threshold determines the routing decision — pass the draft or escalate. The threshold is set empirically by sweeping a benchmark dataset and finding the knee of the accuracy-cost curve.

The known failure mode is confident hallucination: the drafter produces a wrong answer with low entropy. This is mitigated by periodic accuracy audits, downstream feedback loops, and a conservative initial threshold. It's a documented tradeoff, not a bug.

### Speculative Execution

Naive draft-then-verify is serial — hard questions pay double latency. Draft-Thinker fires the heavyweight model in parallel when early tokens show elevated (but not yet critical) uncertainty. If the drafter recovers, the heavyweight call is canceled. If not, the heavyweight already has a head start.

The additional latency on escalated requests is `heavyweight_total - drafter_abort_time`, not the full heavyweight latency.

### Semantic Cache

Previously verified prompt-response pairs are cached via embedding similarity (all-MiniLM-L6-v2 + Qdrant). If an incoming prompt is semantically similar (cosine > 0.95) to a cached entry, the response is returned directly — bypassing the entire draft-verify cycle.

## Tech Stack

- **Gateway** (Go `net/http`): Goroutines for concurrent I/O. The bottleneck is API latency, not compute.
- **Entropy engine** (Go `math`): Pure math — no reason to cross a language boundary.
- **Drafter** (Groq / Together AI): Hosted Llama-3-8B. Fast, cheap, returns logprobs.
- **Heavyweight** (OpenAI / Anthropic): GPT-4o or Claude. Real API costs for honest benchmarking.
- **Vector cache** (Qdrant): Nearest-neighbor lookup for semantic cache.
- **KV store** (Redis): TTLs, metadata, rate counters.
- **Observability** (Prometheus + Grafana): Custom metrics: cost/request, entropy distributions, cache hit rate.
- **Deployment** (Docker Compose): Single command spins up gateway, Redis, Qdrant, Grafana.

No Python in the hot path. The draft-verify state machine is a Go switch statement. LangGraph was considered and rejected — cross-language IPC contradicts the latency story.

## Current State

- **Phase 1 — Foundation**: Proxy with Groq integration (**status:** Complete).
- **Phase 2 — Entropy engine**: Logprob analysis and routing (**status:** In progress).
- **Phase 3 — Calibration**: Threshold sweep and benchmark dataset (**status:** Not started).
- **Phase 4 — Speculative execution**: Parallel heavyweight calls (**status:** Not started).
- **Phase 5 — Semantic cache**: Qdrant + embedding pipeline (**status:** Not started).
- **Phase 6 — Production hardening**: Grafana, load tests, docs (**status:** Not started).

## Metrics (Targets)

These are targets, not claims. Actual numbers will be filled in after calibration (Phase 3) and load testing (Phase 6).

- **TCO reduction**: > 60% vs all-heavyweight baseline.
- **Draft acceptance rate**: > 65% of requests.
- **Accuracy (draft path)**: > 95% acceptable.
- **P99 latency (draft)**: < 200ms.
- **Cache hit rate**: > 15% at steady state.
- **Proxy overhead**: < 5ms P99.

## Quick Start

```bash
# Prerequisites: Go 1.22+, Docker, API keys for Groq + OpenAI/Anthropic

# Clone and build
git clone https://github.com/trnahnh/draft-thinker.git
cd draft-thinker
go build -o draft-thinker ./cmd/gateway

# Start infrastructure
docker compose up -d  # Redis, Qdrant, Grafana

# Run the gateway
export GROQ_API_KEY=...
export OPENAI_API_KEY=...
./draft-thinker --config config.yaml

# Send a request (OpenAI-compatible endpoint)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "auto", "messages": [{"role": "user", "content": "What is 2+2?"}]}'
```

## Documentation

- [System Design](docs/SYSTEM_DESIGN.md) — architecture, entropy algorithm, speculative execution, cache design
- [Development Phases](docs/PHASES.md) — deliverables, exit criteria, and timeline per phase

## Why This Exists

This is a portfolio project targeting Fall 2026 co-op applications at quantitative finance and top tech firms. It demonstrates distributed systems design, production engineering judgment, and the ability to build infrastructure that saves real money — with every claim backed by measured data.

It pairs with [Ferrox](https://github.com/trnahnh/ferrox), a low-latency order matching engine in Rust (500ns P99, 4.7M orders/sec). Together they cover both ends of the systems spectrum: Ferrox is CPU-bound mechanical sympathy; Draft-Thinker is network-bound distributed systems.

## Contact

**Anh Tran** — [anhdtran.forwork@gmail.com](mailto:anhdtran.forwork@gmail.com)

## License

MIT
