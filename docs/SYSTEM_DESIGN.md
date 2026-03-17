# Draft-Thinker: System Design

> Entropy-based intelligent routing for LLM inference cost optimization.

---

## 1. Overview

Draft-Thinker is a stateful reverse proxy that sits between clients and LLM providers. It reduces inference costs by 91.6% by routing requests through a fast, cheap drafter model first, analyzing the drafter's token-level confidence via Shannon entropy, and only escalating to expensive frontier models when the drafter demonstrates high uncertainty.

```text
Client
  │
  ▼
┌─────────────────────────────────────────────────┐
│                  GATEWAY (Go)                   │
│                                                 │
│  ┌───────────┐   ┌──────────┐   ┌───────────┐  │
│  │  Ingress  │──▶│ Semantic │──▶│  Drafter   │  │
│  │  (Auth,   │   │  Cache   │   │   Pool     │  │
│  │  Validate)│   │ (Qdrant) │   │  (OpenAI)  │  │
│  └───────────┘   └────┬─────┘   └─────┬──────┘  │
│                  HIT   │              │ tokens   │
│                   │    │         ┌────▼───────┐  │
│                   │    │         │  Entropy   │  │
│                   │    │         │  Analyzer  │  │
│                   │    │         └────┬───────┘  │
│                   │    │     LOW ┌────┴────┐HIGH │
│                   │    │         │         │     │
│                   ▼    │         ▼         ▼     │
│              ┌────────────┐  ACCEPT   ESCALATE   │
│              │  Response  │◀───┘    ┌────▼────┐  │
│              │  (cached)  │         │Heavy API│  │
│              └─────┬──────┘         │(OpenAI/ │  │
│                    │                │Anthropic│  │
│                    │                └────┬────┘  │
│                    ▼                     ▼       │
│              ┌──────────────────────────────┐    │
│              │         Response             │    │
│              └──────────────────────────────┘    │
└─────────────────────────────────────────────────┘
  │
  ▼
Client
```

---

## 2. Architecture Layers

|Layer|Component|Responsibility|
|-----|--------|--------------|
|**Ingress**|API Gateway|Request validation, auth, rate limiting, request ID generation|
|**Cache**|Semantic Cache|Vector similarity lookup against previously verified responses|
|**Drafting**|Drafter Pool|Routes to fast/cheap model, streams tokens, collects logprobs|
|**Verification**|Entropy Analyzer|Computes per-token and windowed entropy, makes route/escalate decision|
|**Dispatch**|Router|Returns draft or escalates to heavyweight model|
|**Observability**|Prometheus + Grafana|Cost/request, entropy distributions, cache hit rate, escalation %|

---

## 3. Entropy-Based Routing (Core Algorithm)

This is the research anchor. The routing decision is based on Shannon entropy computed from the drafter model's token logprobabilities.

### 3.1 Mechanism

**Token entropy:** For each generated token, the drafter returns logprobs for the top-k candidates. Compute:

```text
H = -Σ p(x) log₂ p(x)
```

Low H = model is confident. High H = model is uncertain.

**Windowed entropy:** Rather than checking every individual token, compute a sliding window average (window size = 10 tokens). This smooths noise from individual uncertain tokens (e.g., rare proper nouns) that don't indicate reasoning failure.

**Threshold decision:** If windowed entropy exceeds a calibrated threshold `T` at any point during generation, flag the request for escalation. `T` is determined empirically during calibration.

**Early exit:** If the first N tokens (N=10) show entropy above `T`, abort the draft immediately and escalate. Don't waste compute finishing a draft you'll discard.

### 3.2 Why Entropy Over Prompt Classification

Prompt classifiers ("is this question easy or hard?") fail on distribution shift. A question that looks simple syntactically ("What's the current Fed rate?") may require complex reasoning depending on context. Entropy-based routing measures **actual model confidence during generation**, not predicted difficulty before generation. This makes it robust to novel query patterns.

### 3.3 Known Failure Mode: Confident Hallucination

The drafter can produce confidently wrong answers (low entropy, bad output). This is the fundamental limitation of entropy-based routing. Mitigations:

1. Periodic accuracy audits on draft-accepted responses
2. Downstream feedback loop for flagging bad responses
3. Conservative initial threshold (err toward escalation)
4. Accept as a tradeoff -- optimizing cost, not eliminating errors

---

## 4. Speculative Execution

### 4.1 Problem

Naive draft-then-verify is serial. Hard questions incur double latency: drafter time + heavyweight time.

### 4.2 Solution

```text
Drafter starts generating
  │
  ├── First 10 tokens: windowed entropy > 0.8 * T (soft threshold)
  │     │
  │     └── Fire parallel request to heavyweight model
  │
  ├── Drafter entropy drops back below T → Cancel heavyweight (recovered)
  │
  └── Drafter entropy stays high → Heavyweight already has head start
        │
        └── Additional latency = heavyweight_total - drafter_abort_time
            (NOT full heavyweight latency)
```

### 4.3 Request Lifecycle State Machine

```text
         ┌──────────┐
         │ DRAFTING  │
         └────┬──────┘
              │
     entropy > 0.8*T?
        ┌─────┴─────┐
        │ NO        │ YES
        ▼           ▼
  ┌──────────┐ ┌─────────────┐
  │ DRAFT    │ │ SPECULATING │
  │ ACCEPTED │ └──────┬──────┘
  └──────────┘        │
               entropy < T?
              ┌───────┴───────┐
              │ YES           │ NO
              ▼               ▼
        ┌──────────┐   ┌───────────┐
        │ DRAFT    │   │ ESCALATED │
        │ ACCEPTED │   │ (heavy    │
        │ (cancel  │   │  response)│
        │  heavy)  │   └───────────┘
        └──────────┘
```

### 4.4 Cost Tradeoff

Speculative execution trades a small amount of wasted compute (~5–10% of escalated request costs) for significantly better tail latency. Net TCO impact is minimal: only ~30% of requests trigger escalation, and of those, the speculative call is canceled ~40% of the time (drafter recovers).

---

## 5. Semantic Cache Layer

### 5.1 Purpose

Bypass the entire draft-verify cycle for previously seen reasoning patterns.

### 5.2 Architecture

```text
Request arrives
  │
  ▼
Embed prompt (all-MiniLM-L6-v2, 384-dim)
  │
  ▼
Qdrant nearest-neighbor lookup
  │
  ├── cosine similarity > 0.95 → Return cached response (< 50ms)
  │
  └── similarity ≤ 0.95 → Proceed to drafter pipeline
```

### 5.3 Design Decisions

|Decision|Choice|Rationale|
|--------|------|---------|
|**Embedding model**|all-MiniLM-L6-v2|384-dim, fast enough to run inline without meaningful latency|
|**Similarity threshold**|0.95|Intentionally conservative to avoid stale/drifted answers|
|**Vector store**|Qdrant|Self-hosted, lightweight, purpose-built for nearest-neighbor|
|**Metadata store**|Redis|TTLs, eviction tracking, rate counters|
|**Cache key**|Prompt embedding|Semantic similarity, not exact match|
|**Invalidation**|TTL + manual eviction API|Entries expire by time; bad responses evicted immediately via API|

### 5.4 Cache Isolation

The cache lookup runs on the request's hot path but the vector store query is separate from the main proxy logic. On cache miss, the system proceeds to drafting with zero added latency beyond the embedding + lookup time (~10–15ms).

---

## 6. Technology Stack

|Layer|Technology|Rationale|
|-----|----------|---------|
|**Gateway**|Go (`net/http`)|Goroutines for concurrent request handling. No framework overhead. Native HTTP/2.|
|**Entropy Engine**|Go (`math`)|Entropy computation is pure math, no need to cross language boundary.|
|**Drafter Models**|OpenAI (gpt-4.1-nano)|Fast, cheap, returns logprobs.|
|**Heavyweight**|OpenAI (gpt-4.1)|Escalation target. Real API costs for benchmarking.|
|**Cache (KV)**|Redis|Metadata, TTLs, rate counters.|
|**Cache (Vector)**|Qdrant|Nearest-neighbor lookup for semantic cache. Self-hosted, lightweight.|
|**Embedding**|all-MiniLM-L6-v2|384-dim embeddings. Fast enough to run inline.|
|**Observability**|Prometheus + Grafana|Custom metrics: cost/request, entropy distributions, cache hit rate, escalation %.|
|**Deployment**|Docker Compose|Single-command local dev. Compose handles gateway, Redis, Qdrant, Grafana.|

### Why Go End-to-End (No Python)

The original concept used Go for the gateway and Python/LangGraph for orchestration. This design eliminates the Python dependency entirely. The draft-verify state machine is simple enough to implement natively in Go (a switch statement over request states), and cross-language IPC would add latency that contradicts the project's core value proposition. Go's goroutines handle the concurrent speculative execution pattern naturally.

---

## 7. Key Metrics

|Metric|Target|Measurement|
|------|------|-----------|
|TCO Reduction|91.6% vs all-heavyweight baseline|Calibrated on 518 prompts at T=2.0|
|Draft Acceptance Rate|94% of requests|Requests served by drafter / total requests|
|Accuracy (Draft Path)|98.2% acceptable|LLM-as-judge evaluation on benchmark set|
|P99 Latency (Draft)|< 200ms|Prometheus histogram|
|P99 Latency (Escalated)|< 2x heavyweight standalone|Compared to direct heavyweight call|
|Cache Hit Rate|> 15% at steady state|Cache hits / total requests over 1hr window|
|Proxy Overhead|< 5ms P99|Gateway processing time excluding model inference|

---

## 8. Data Flow (End-to-End)

```text
1. Client sends POST /v1/chat/completions
2. Ingress: validate request, assign request_id, check rate limits
3. Cache lookup: embed prompt → Qdrant similarity search
   ├── HIT (sim > 0.95): return cached response, log cache_hit metric
   └── MISS: continue
4. Draft: forward to OpenAI (gpt-4.1-nano) with logprobs=true, stream=true
5. Entropy analysis (per-token, streaming):
   ├── All tokens below T: DRAFT_ACCEPTED → return response, cache it
   ├── Early tokens > 0.8*T: fire speculative heavyweight call
   │   ├── Entropy drops below T: cancel heavyweight → DRAFT_ACCEPTED
   │   └── Entropy stays above T: abort draft → return heavyweight response
   └── Early tokens > T: abort draft immediately → full escalation
6. Log metrics: cost, latency, entropy, routing decision, cache status
```
