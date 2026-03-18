# Draft-Thinker: Resume Bullets

Pick 2-3 bullets for your resume. All numbers reference measured data.

## Bullet Options

- Built a cost-aware LLM gateway in Go that reduces inference costs by 91.6%
by routing 94% of requests through a drafter model using real-time Shannon
entropy analysis of token logprobabilities, with 98.2% accuracy on a
518-prompt calibration benchmark.
- Designed speculative execution for LLM routing using goroutine-based parallel
dispatch with a soft entropy threshold (0.8*T), reducing tail latency on
escalated requests while keeping wasted compute under 10% of escalation cost.
- Implemented a semantic cache layer using OpenAI embeddings, Qdrant vector
search, and Redis TTL that serves repeated queries in < 50ms with cosine
similarity > 0.95 threshold.
- Engineered an entropy-based routing system that measures drafter model confidence
during token generation rather than classifying prompts upfront, making routing
robust to distribution shift across 4 query categories (factual, reasoning,
code, creative).
- Built a 12-panel Grafana dashboard auto-provisioned via Docker Compose for
real-time observability of cost savings, latency percentiles, entropy
distributions, cache hit rate, and speculative execution metrics across
11 Prometheus instruments.

## Interview Talking Points

- Threshold calibration methodology: swept T=1.0 to 2.5, selected T=2.0 based
on F1 score with a 95% draft accuracy gate. 518 prompts, LLM-as-judge eval.
- State machine: DRAFTING -> SPECULATING -> ESCALATED | DRAFT_ACCEPTED.
Soft threshold (0.8*T) fires heavyweight in parallel via goroutine.
- Why entropy over prompt classification: classifiers fail on distribution shift.
A syntactically simple question can require complex reasoning depending on
context. Entropy measures actual model confidence during generation.
- Confident hallucination as an accepted tradeoff: the drafter can produce wrong
answers with low entropy. Mitigated by conservative threshold, periodic audits,
and downstream feedback. Optimizing cost, not eliminating all errors.
- Conservative cache threshold (cosine > 0.95): better to miss cache and re-draft
than to return a stale or wrong cached answer.
- Go over Python/LangGraph: the draft-verify state machine is a switch statement.
Cross-language IPC adds latency that contradicts the cost/latency value prop.
Goroutines handle speculative execution naturally.
- Semantic cache only caches draft-accepted responses. Escalated responses
indicate drafter uncertainty and are not safe to cache.
