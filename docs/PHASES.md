# Draft-Thinker: Development Phases

> **Timeline:** 8 weeks | **Language:** Go | **Codename:** Draft-Thinker

---

## Phase 1 -Foundation (Week 1-2) [COMPLETE]

**Goal:** Build the basic proxy that forwards requests to a drafter model and returns responses. No routing logic yet.

### Phase 1 deliverables

- Go HTTP server accepting OpenAI-compatible `/v1/chat/completions` requests
- Request validation and structured error handling
- Forwarding layer to OpenAI API (gpt-4.1-nano) with streaming support
- Response passthrough back to client with latency measurement
- Docker Compose with the gateway container
- Basic Prometheus metrics: request count, latency histogram, error rate

### Phase 1 exit criteria

A client can send a chat completion request to the gateway and receive a streamed response from the drafter model. Latency overhead from the proxy layer is **< 5ms P99**.

---

## Phase 2 -Entropy Engine (Week 3-4) [COMPLETE]

**Goal:** Implement the entropy computation pipeline and make the first routing decisions.

### Phase 2 deliverables

- Logprob extraction from drafter responses (OpenAI returns these natively)
- Per-token Shannon entropy computation: `H = -Σ p(x) log p(x)`
- Sliding window entropy with configurable window size
- Threshold-based routing decision: pass (return draft) or escalate
- Escalation path to OpenAI heavyweight model (gpt-4.1)
- Prometheus metrics: entropy distribution histogram, escalation rate, cost per request

### Phase 2 exit criteria

The gateway routes requests to either the drafter or heavyweight model based on entropy. Routing decisions are observable in Grafana and correlate entropy scores with response quality.

---

## Phase 3 -Calibration & Benchmarking (Week 5) [COMPLETE]

**Goal:** Empirically determine the optimal entropy threshold and produce defensible metrics.

### Phase 3 deliverables

- **Benchmark dataset:** 520 prompts across 4 categories (simple factual, multi-step reasoning, code generation, ambiguous/creative)
- **Ground truth labels:** each prompt evaluated by LLM-as-judge against heavyweight reference answer
- **Threshold sweep:** dataset swept at T = 1.0, 1.25, 1.5, 1.75, 2.0, 2.25, 2.5
- **Accuracy-cost tradeoff curve:** escalation rate vs. draft accuracy at each threshold
- Final threshold selection: T=2.0

### Phase 3 exit criteria

> "At threshold T=2.0, the system routes 94% of requests to the drafter with 98.2% accuracy, reducing cost by 91.6% vs. baseline."

---

## Phase 4 -Speculative Execution (Week 6) [COMPLETE]

**Goal:** Eliminate the latency penalty on escalated requests.

### Phase 4 deliverables

- Soft threshold (`0.8 * T`) detection during token streaming
- Parallel heavyweight request initiation via goroutine
- Cancellation logic: abort heavyweight if drafter recovers
- Request lifecycle state machine:

  ```text
  DRAFTING -> SPECULATING -> ESCALATED | DRAFT_ACCEPTED
  ```

- Metrics: speculative trigger rate, cancellation rate, latency saved on escalated requests
- Config: `speculative.enabled` (default true), `speculative.soft_threshold_mult` (default 0.8)
- Graceful fallback: if speculative heavyweight fails to start, continues in drafting state
- Serial path preserved when `speculative.enabled: false`

### Phase 4 exit criteria

Escalated requests show measurably lower P99 latency compared to Phase 2's serial approach. Wasted compute from canceled speculative calls is **< 10%** of total escalation cost.

---

## Phase 5 -Semantic Cache (Week 7) [COMPLETE]

**Goal:** Cache verified responses to skip the draft-verify cycle on repeated reasoning patterns.

### Phase 5 deliverables

- Prompt embedding via OpenAI text-embedding-3-small (1536-dim, hosted -consistent with project philosophy)
- Qdrant integration for vector similarity search (raw HTTP, 3 endpoints)
- Redis via go-redis/v9 for response storage with TTL
- Cache insertion on verified draft-accepted responses only (escalated responses are not cached)
- Cache lookup on request ingress (before drafting), supports both JSON and SSE responses
- Conservative similarity threshold (cosine > 0.95) -better to miss cache than return wrong answer
- TTL-based expiration with lazy cleanup of orphaned Qdrant points
- Manual eviction API: DELETE /v1/cache/{id}
- Cache disabled by default (*bool nil = false), requires Redis + Qdrant infrastructure
- Metrics: cache hit rate, cache miss rate, lookup latency histogram

### Phase 5 exit criteria

Repeated or semantically similar queries return cached responses with **< 50ms latency**. Cache hit rate is measurable and tracked in Grafana.

---

## Phase 6 -Production Hardening & Documentation (Week 8) [COMPLETE]

**Goal:** Make the project interview-ready and publicly presentable.

### Phase 6 deliverables

- **Grafana dashboard:** 12-panel dashboard auto-provisioned via Docker Compose. Covers request rate, draft acceptance rate, cost reduction, cache hit rate, latency percentiles (upstream, cache lookup, speculative saved), routing decisions over time, error rate, entropy distribution heatmap, speculative trigger/cancellation rates.
- **Load testing** with vegeta: mock OpenAI SSE server for throughput testing (confident, mixed, cache scenarios). Measures P50/P95/P99 latency, throughput, and error rate under configurable concurrency.
- **README** updated with observability section, architecture references, and benchmark results.
- **Technical writeup:** SYSTEM_DESIGN.md updated with correct embedding model (text-embedding-3-small) and current architecture.
- **Resume bullets** in docs/RESUME.md with measured data and interview talking points.

### Phase 6 exit criteria

The project is deployed, documented, and you can walk an interviewer through every design decision with supporting data. `docker compose up -d` provisions the full stack including Grafana dashboard automatically.

---

## Timeline Summary

|Week|Phase|Key Deliverable|
|---|-----|---------------|
|1-2|Foundation|Working proxy with OpenAI integration|
|3-4|Entropy Engine|Routing decisions based on token entropy|
|5|Calibration|Threshold selected with accuracy-cost curve|
|6|Speculative Execution|Parallel escalation with cancellation|
|7|Semantic Cache|Vector-based response caching|
|8|Hardening|Dashboard, load tests, documentation|

**Note:** Parallelizable with Ferrox development if phases are staggered. Phase 1–2 can overlap with Ferrox Phase 5–6.
