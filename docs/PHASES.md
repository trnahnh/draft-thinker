# Draft-Thinker: Development Phases

> **Timeline:** 8 weeks | **Language:** Go | **Codename:** Draft-Thinker

---

## Phase 1 — Foundation (Week 1–2) [COMPLETE]

**Goal:** Build the basic proxy that forwards requests to a drafter model and returns responses. No routing logic yet.

### Phase 1 deliverables

- Go HTTP server accepting OpenAI-compatible `/v1/chat/completions` requests
- Request validation and structured error handling
- Forwarding layer to Groq API (Llama-3-8B) with streaming support
- Response passthrough back to client with latency measurement
- Docker Compose with the gateway container
- Basic Prometheus metrics: request count, latency histogram, error rate

### Phase 1 exit criteria

A client can send a chat completion request to the gateway and receive a streamed response from the drafter model. Latency overhead from the proxy layer is **< 5ms P99**.

---

## Phase 2 — Entropy Engine (Week 3–4) [COMPLETE]

**Goal:** Implement the entropy computation pipeline and make the first routing decisions.

### Phase 2 deliverables

- Logprob extraction from drafter responses (Groq returns these natively)
- Per-token Shannon entropy computation: `H = -Σ p(x) log p(x)`
- Sliding window entropy with configurable window size
- Threshold-based routing decision: pass (return draft) or escalate
- Escalation path to OpenAI/Anthropic API
- Prometheus metrics: entropy distribution histogram, escalation rate, cost per request

### Phase 2 exit criteria

The gateway routes requests to either the drafter or heavyweight model based on entropy. Routing decisions are observable in Grafana and correlate entropy scores with response quality.

---

## Phase 3 — Calibration & Benchmarking (Week 5)

**Goal:** Empirically determine the optimal entropy threshold and produce defensible metrics.

### Phase 3 deliverables

- **Benchmark dataset:** 500+ prompts across difficulty tiers (simple factual, multi-step reasoning, code generation, ambiguous/creative)
- **Ground truth labels:** each prompt labeled with whether the drafter's answer was acceptable (human-evaluated or LLM-as-judge)
- **Threshold sweep:** run the dataset through the gateway at `T = 0.5, 0.75, 1.0, 1.25, 1.5` and measure accuracy vs. cost tradeoff
- **ROC curve:** plot escalation rate vs. accuracy at each threshold
- Final threshold selection with documented rationale

### Phase 3 exit criteria

A calibrated threshold exists with a documented accuracy-cost tradeoff curve. You can state with evidence:

> "At threshold T=X, the system routes Y% of requests to the drafter with Z% accuracy, reducing cost by W% vs. baseline."

---

## Phase 4 — Speculative Execution (Week 6)

**Goal:** Eliminate the latency penalty on escalated requests.

### Phase 4 deliverables

- Soft threshold (`0.8 * T`) detection during token streaming
- Parallel heavyweight request initiation via goroutine
- Cancellation logic: abort heavyweight if drafter recovers
- Request lifecycle state machine:

  ```text
  DRAFTING → SPECULATING → ESCALATED | DRAFT_ACCEPTED
  ```

- Metrics: speculative execution trigger rate, cancellation rate, latency improvement on escalated requests

### Phase 4 exit criteria

Escalated requests show measurably lower P99 latency compared to Phase 2's serial approach. Wasted compute from canceled speculative calls is **< 10%** of total escalation cost.

---

## Phase 5 — Semantic Cache (Week 7)

**Goal:** Cache verified responses to skip the draft-verify cycle on repeated reasoning patterns.

### Phase 5 deliverables

- Prompt embedding pipeline (all-MiniLM-L6-v2, running locally or via API)
- Qdrant integration for vector similarity search
- Cache insertion on verified draft responses
- Cache lookup on request ingress (before drafting)
- TTL-based expiration and manual eviction API
- Metrics: cache hit rate, latency on cache hits vs. draft path

### Phase 5 exit criteria

Repeated or semantically similar queries return cached responses with **< 50ms latency**. Cache hit rate is measurable and tracked in Grafana.

---

## Phase 6 — Production Hardening & Documentation (Week 8)

**Goal:** Make the project interview-ready and publicly presentable.

### Phase 6 deliverables

- **Grafana dashboard:** cost savings, entropy distributions, cache hit rate, latency percentiles, escalation breakdown
- **Load testing** with k6 or vegeta: measure throughput and latency under concurrency
- **README** with architecture diagram, setup instructions, and benchmark results
- **Technical writeup** explaining the entropy-based routing approach
- **Resume bullets** backed by measured data

### Phase 6 exit criteria

The project is deployed, documented, and you can walk an interviewer through every design decision with supporting data.

---

## Timeline Summary

|Week|Phase|Key Deliverable|
|---|-----|---------------|
|1–2|Foundation|Working proxy with Groq integration|
|3–4|Entropy Engine|Routing decisions based on token entropy|
|5|Calibration|Threshold selected with accuracy-cost curve|
|6|Speculative Execution|Parallel escalation with cancellation|
|7|Semantic Cache|Vector-based response caching|
|8|Hardening|Dashboard, load tests, documentation|

**Note:** Parallelizable with Ferrox development if phases are staggered. Phase 1–2 can overlap with Ferrox Phase 5–6.
