# Draft-Thinker

A cost-aware LLM gateway in Go that reduces inference costs by routing requests through entropy-based draft-and-verify logic. Cheap model drafts first, expensive model only when the drafter isn't confident.

## The Problem

LLM-powered applications send 100% of traffic to frontier models regardless of query complexity. "What are your hours?" costs the same as "Explain the tradeoffs between B-tree and LSM-tree storage engines." This is wasteful in three ways:

- **Cost**: 70%+ of queries are answerable by models costing 10-50x less.
- **Latency**: Frontier models have 2-5x higher time-to-first-token than small models.
- **Scale**: At high throughput, frontier model rate limits become the bottleneck, not your application.

The hard part isn't routing -- it's knowing *when* the cheap model is good enough without already having the right answer. Prompt classifiers ("is this question easy?") fail on distribution shift. A syntactically simple question can require complex reasoning depending on context.

## The Approach

Draft-Thinker solves this by analyzing the drafter model's own confidence signals during generation. Every token a model produces comes with log-probabilities for its top candidates. High entropy (uncertainty) across those candidates means the model is guessing. Low entropy means it's confident.

The gateway watches these signals in real-time as the drafter generates. If confidence stays high, ship the draft. If it drops, escalate to the heavyweight.

### Entropy-Based Routing

The core mechanism computes Shannon entropy over the drafter's token logprobabilities using a sliding window. A calibrated threshold determines the routing decision: pass the draft or escalate. The threshold is set empirically by sweeping a benchmark dataset and finding the knee of the accuracy-cost curve.

The known failure mode is confident hallucination: the drafter produces a wrong answer with low entropy. This is mitigated by periodic accuracy audits, downstream feedback loops, and a conservative initial threshold. It's a documented tradeoff, not a bug.

### Speculative Execution

Naive draft-then-verify is serial. Hard questions pay double latency. Draft-Thinker fires the heavyweight model in parallel when early tokens show elevated (but not yet critical) uncertainty. If the drafter recovers, the heavyweight call is canceled. If not, the heavyweight already has a head start.

The additional latency on escalated requests is `heavyweight_total - drafter_abort_time`, not the full heavyweight latency.

### Semantic Cache

Previously verified prompt-response pairs are cached via embedding similarity (OpenAI text-embedding-3-small + Qdrant). If an incoming prompt is semantically similar (cosine > 0.95) to a cached entry, the response is returned directly, bypassing the entire draft-verify cycle. Only draft-accepted responses are cached -- escalated responses indicate drafter uncertainty and are not safe to cache.

## Tech Stack

- **Gateway** (Go `net/http`): Goroutines for concurrent I/O. The bottleneck is API latency, not compute.
- **Entropy engine** (Go `math`): Pure math, no reason to cross a language boundary.
- **Drafter** (OpenAI gpt-4.1-nano): Fast, cheap, returns logprobs.
- **Heavyweight** (OpenAI gpt-4.1): Capable model for escalation. Real API costs for honest benchmarking.
- **Vector cache** (Qdrant): Nearest-neighbor lookup for semantic cache.
- **KV store** (Redis): TTLs, metadata, rate counters.
- **Observability** (Prometheus + Grafana): Custom metrics: cost/request, entropy distributions, cache hit rate.
- **Deployment** (Docker Compose): Single command spins up gateway, Redis, Qdrant, Grafana.

No Python in the hot path. The draft-verify state machine is a Go switch statement. LangGraph was considered and rejected. Cross-language IPC contradicts the latency story.

## Current State

- **Phase 1 -- Foundation**: Proxy with OpenAI integration (**status:** Complete).
- **Phase 2 -- Entropy engine**: Logprob analysis and routing (**status:** Complete).
- **Phase 3 -- Calibration**: Threshold sweep and benchmark dataset (**status:** Complete).
- **Phase 4 -- Speculative execution**: Parallel heavyweight calls (**status:** Complete).
- **Phase 5 -- Semantic cache**: Qdrant + embedding pipeline (**status:** Complete).
- **Phase 6 -- Production hardening**: Grafana, load tests, docs (**status:** Complete).

## Metrics

Calibrated on 518 prompts across 4 categories (simple factual, multi-step reasoning, code generation, ambiguous/creative) using LLM-as-judge evaluation.

- **TCO reduction**: 91.6% vs all-heavyweight baseline (at T=2.0).
- **Draft acceptance rate**: 94% of requests served by drafter.
- **Accuracy (draft path)**: 98.2% acceptable (LLM-as-judge).
- **Calibrated threshold**: T=2.0 (Shannon entropy in bits, 10-token sliding window).
- **P99 latency (draft)**: 109ms at 50 req/s (vegeta, mock upstream, 100% success).
- **Cache hit rate**: workload-dependent (repeated queries hit cache, unique queries do not).
- **Proxy overhead**: < 5ms P99.

## Quick Start

```bash
# Prerequisites: Go 1.22+, Docker, OpenAI API key

# Clone and build
git clone https://github.com/trnahnh/draft-thinker.git
cd draft-thinker
go build -o draft-thinker ./cmd/gateway

# Start infrastructure
docker compose up -d  # Redis, Qdrant, Grafana

# Run the gateway
export OPENAI_API_KEY=...
./draft-thinker --config config.yaml

# Send a request (OpenAI-compatible endpoint)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "auto", "messages": [{"role": "user", "content": "What is 2+2?"}]}'
```

## Demo

Research showcase site presenting calibration results, entropy analysis, and architecture.

**Live:** [draft-thinker.vercel.app](https://draft-thinker.vercel.app)

Run locally:

```bash
cd web
npm install
npm run dev
```

## Observability

`docker compose up -d` auto-provisions Grafana with a pre-built dashboard at `http://localhost:3000` (admin/admin). The dashboard covers request rates, draft acceptance, latency percentiles, routing decisions, entropy distribution, cache hit rate, and speculative execution metrics. Prometheus scrapes the gateway at `/metrics` every 15 seconds.

## Documentation

- [System Design](docs/SYSTEM_DESIGN.md) -- architecture, entropy algorithm, speculative execution, cache design
- [Development Phases](docs/PHASES.md) -- deliverables, exit criteria, and timeline per phase
- [Metrics](docs/METRICS.md) -- metrics exposed by the gateway at the configured metrics path
- [Resume Bullets](docs/RESUME.md) -- interview-ready project summary with measured data

## Inspired By

This project is inspired by the research [Draft-Thinking: Learning Efficient Reasoning in Long Chain-of-Thought LLMs](https://arxiv.org/abs/2603.00578).

## Why This Exists

DraftThinker demonstrates distributed systems design, production engineering judgment, and the ability to build infrastructure that saves real money, with every claim backed by measured data.

It pairs with [Ferrox](https://github.com/trnahnh/ferrox), a low-latency order matching engine in Rust (500ns P99, 4.7M orders/sec). Together they cover both ends of the systems spectrum: Ferrox is CPU-bound mechanical sympathy; Draft-Thinker is network-bound distributed systems.

## Contact

**Anh Tran** -- [anhdtran.forwork@gmail.com](mailto:anhdtran.forwork@gmail.com)

## License

MIT
