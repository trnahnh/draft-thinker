# Draft-Thinker: System Design

> Entropy-based intelligent routing for LLM inference cost optimization.

---

## 1. Overview

Draft-Thinker is a stateful reverse proxy that sits between clients and LLM providers. It routes requests through a fast, cheap drafter model first, analyzes the drafter's token-level confidence via Shannon entropy, and escalates to an expensive frontier model when the drafter demonstrates high uncertainty. On the committed 519-prompt benchmark this cuts measured inference cost by 82.9% (95% CI 79.6% to 86.0%) at 98.1% served accuracy; see `docs/RESULTS.md` for the baselines that qualify that number.

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
│              └─────┬──────┘         │(OpenAI) │  │
│                    │                │         │  │
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

The drafter can produce confidently wrong answers (low entropy, bad output). This is the fundamental limitation of entropy-based routing, and it is measured rather than hypothetical: all 11 unacceptable drafts in the project benchmark have mean entropy between 0.10 and 0.48 bits, and the router at T=2.0 catches 1 of them. Mitigations:

1. Periodic accuracy audits on draft-accepted responses
2. Downstream feedback loop for flagging bad responses
3. Conservative initial threshold (err toward escalation)
4. Accept as a tradeoff -optimizing cost, not eliminating errors

### 3.4 What the Evaluation Showed About the Statistic

`docs/RESULTS.md` evaluates the windowed peak statistic (the exact value this router thresholds) as a detector of wrong drafts on three datasets. It has AUROC 0.68 on the project set but 0.51 on GSM8K and 0.47 on MMLU, where answer keys replace the LLM judge. At the production threshold the router is statistically indistinguishable from escalating the same fraction of requests at random. Trace-level statistics that need the complete draft (mean entropy: AUROC 0.76 on GSM8K, 0.63 on MMLU; minimum logprob: 0.62 and 0.68) carry real signal. The design tension is that the windowed statistic is what makes early exit and speculative execution possible; a running mean with a minimum token count is the candidate replacement.

---

## 4. Speculative Execution

### 4.1 Problem

Naive draft-then-verify is serial. Hard questions incur double latency: drafter time + heavyweight time.

### 4.2 Solution

```text
Drafter starts generating
  │
  ├── Any single token with entropy > 0.8 * T (soft threshold)
  │     │
  │     └── Fire parallel request to heavyweight model
  │
  ├── Drafter finishes without the window crossing T → Cancel heavyweight
  │
  └── Window crosses T → Heavyweight already has head start
        │
        └── Additional latency = heavyweight_total - drafter_abort_time
            (NOT full heavyweight latency)
```

As implemented (`internal/speculative/executor.go`) the soft trigger is a
per-token test, not a windowed one, and the hard-threshold check runs
before the soft check on each token, so a token that jumps straight past T
escalates without any speculative call. Both facts matter for the measured
behaviour in 4.4.

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

Speculative execution trades wasted compute on cancelled heavyweight calls for better tail latency on true escalations. Measured by replaying the recorded traces through this executor (`benchmarks/cmd/replayspec`, `docs/RESULTS.md` section 11), at the production multiplier 0.8 it fires on 70.5% of project-set requests and cancels 96.4% of those calls; only 13 of the 40 escalations get a head start, with a median head start of 36 ms. The same holds on every other trace (86 to 89% trigger, 93 to 99% cancel). If cancelled calls were billed to completion they would cost 80 to 88% of the always-heavyweight bill, more than the router saves; what they actually bill depends on provider behaviour and was not measured. The Phase 4 target of under 10% waste is not met at any multiplier from 0.5 to 0.9. The practical conclusion is that speculative execution should be off (`speculative.enabled: false`) until a prefix signal exists; see `docs/STREAMING_DESIGN.md`.

---

## 5. Semantic Cache Layer

### 5.1 Purpose

Bypass the entire draft-verify cycle for previously seen reasoning patterns.

### 5.2 Architecture

```text
Request arrives
  │
  ▼
Embed prompt (text-embedding-3-small, 1536-dim)
  │
  ▼
Qdrant nearest-neighbor lookup
  │
  ├── cosine similarity > 0.95 → Return cached response (about 200 ms, dominated by the embedding call)
  │
  └── similarity ≤ 0.95 → Proceed to drafter pipeline
```

### 5.3 Design Decisions

|Decision|Choice|Rationale|
|--------|------|---------|
|**Embedding model**|text-embedding-3-small|1536-dim via OpenAI API, consistent with hosted model philosophy|
|**Similarity threshold**|0.95|Intentionally conservative to avoid stale/drifted answers|
|**Vector store**|Qdrant|Self-hosted, lightweight, purpose-built for nearest-neighbor|
|**Metadata store**|Redis|TTLs, eviction tracking, rate counters|
|**Cache key**|Prompt embedding|Semantic similarity, not exact match|
|**Invalidation**|TTL + manual eviction API|Entries expire by time; bad responses evicted immediately via API|

### 5.4 Cache Isolation

The cache lookup runs on the request's hot path but the vector store query is separate from the main proxy logic. On cache miss, the system proceeds to drafting after paying the embedding call plus lookup, measured at p50 197 to 201 ms and p99 360 to 406 ms by replaying the traces through `cache.VectorStore` (`benchmarks/cmd/replaycache`, `docs/RESULTS.md` section 12). Measured hit rate on unique benchmark traffic is 0% on the project set and GSM8K and 1.0% on MMLU, where one of the three hits served a wrong answer to a templated near-duplicate at cosine 0.959; at 0.90 the same replay would serve 19 wrong answers in 30 hits, which is the measured case for keeping 0.95.

---

## 5A. Failure Handling

Provider calls fail in two places, and the gateway degrades differently in each (`internal/gateway/handler.go`, configured under `fallback:`):

|Failure|Default behaviour|Response headers|
|-------|-----------------|----------------|
|Drafter errors or times out before a decision|Send the request straight to the heavyweight|`X-Routing-Decision: fallback_heavyweight`, `X-Fallback-Reason: drafter_error`|
|Heavyweight errors after an escalation decision|Re-run the drafter to completion and serve its answer, uncached|`X-Routing-Decision: fallback_draft`, `X-Fallback-Reason: heavyweight_error`|
|Both fail|Surface the heavyweight error with its upstream status|none|

Streaming escalations wait for the first heavyweight chunk before committing to an SSE response so that an immediate failure can still fall back; once bytes are on the wire the stream is terminated with `[DONE]`. Fallback responses are never inserted into the semantic cache. The drafter is re-run rather than served from the partial draft because escalation aborts the drafter stream, so the collected draft is usually incomplete. Each fallback increments `draftthinker_errors_total{type="drafter_fallback"|"heavyweight_fallback"}` and the routing-decision counter, so a provider incident shows up on the dashboard as a shift in decision mix rather than as a wall of 5xx. `benchmarks/cmd/loadtest -scenario flaky` injects drafter 503s and stalls against the mock upstream to exercise this path.

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
|**Embedding**|OpenAI text-embedding-3-small|1536-dim embeddings via API. Consistent with hosted model philosophy.|
|**Observability**|Prometheus + Grafana|Custom metrics: cost/request, entropy distributions, cache hit rate, escalation %.|
|**Deployment**|Docker Compose|Single-command local dev. Compose handles gateway, Redis, Qdrant, Grafana.|

### Why Go End-to-End (No Python)

The original concept used Go for the gateway and Python/LangGraph for orchestration. This design eliminates the Python dependency entirely. The draft-verify state machine is simple enough to implement natively in Go (a switch statement over request states), and cross-language IPC would add latency that contradicts the project's core value proposition. Go's goroutines handle the concurrent speculative execution pattern naturally.

---

## 7. Key Metrics

Measured values come from the commands in `docs/RESULTS.md` section 14 and `README.md`; targets that have not been measured are marked as such.

|Metric|Measured|Source|
|------|--------|------|
|Cost reduction vs all-heavyweight|82.9% (95% CI 79.6% to 86.0%) at T=2.0|`benchmarks/results/compare.json`, 519 prompts, input and output tokens priced|
|Draft acceptance rate|92.3% (479 of 519)|same|
|Served accuracy|98.1% (95% CI 96.7% to 99.0%), LLM-as-judge|same|
|Rate-matched random baseline|98.1% accuracy at 83.9% cost reduction|same; the router does not beat it|
|GSM8K / MMLU served accuracy at T=2.0|93.0% / 77.3% (always-drafter 92.7% / 75.7%)|`compare_gsm8k.json`, `compare_mmlu.json`|
|Cost per 1,000 requests at T=2.0|USD 0.534 (project set), 0.293 (GSM8K), 0.549 (MMLU) vs 3.127 / 1.866 / 2.215 all-heavyweight|same|
|Upstream latency, serial path, estimated from recorded calls|p50 3.7 s, p99 19.7 s on the project set (drafter and heavyweight round trips are similar at ~290 output tokens)|same|
|Gateway latency under load (mock upstream)|see README load-test table|`benchmarks/cmd/loadtest`|
|Cache hit rate on replayed unique benchmark traffic|0.0% project set, 0.0% GSM8K, 1.0% MMLU (1 of 3 hits wrong); lookup p50 about 200 ms|`benchmarks/results/cache_replay_*.json`|
|Speculative trigger and cancellation rates (replay through the executor)|70.5% trigger, 96.4% cancel on the project set; 86 to 89% and 93 to 99% on the other traces|`benchmarks/results/speculative_*.json`|
|Replication on disjoint samples (GSM8K, MMLU, ARC-Challenge, n=500 each)|window-peak AUROC 0.57 / 0.45 / 0.52; router 0.2 to 1.0 points below rate-matched random|`benchmarks/results/compare_*_r2.json`|
|Learned whole-trace router (trained round 1, tested round 2)|AUROC 0.81 / 0.72 / 0.75 (GSM8K, MMLU, ARC), equal to mean entropy alone within CIs|same|

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
