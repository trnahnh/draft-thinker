# Enhancement Report: evaluation hardening and failure handling

Round 1 of two. Round 2 (replication on new samples, a learned router,
measured speculative execution and cache, open-model accuracy) is in
`ROUND2_REPORT.md` on branch `round2/draft-thinker`.

Branch: `enhance/evaluation-hardening` (built on `main` at `001889b`). Every
number in this report is produced by a command listed next to it, from data
committed on the branch. API spend for this pass: under USD 2 (GSM8K and
MMLU collection, 600 prompts, gpt-4.1-nano plus gpt-4.1). The open-model run
used a local Ollama server and cost nothing.

## 1. Audit findings

Architecture (Go gateway, entropy window, speculative executor, semantic
cache, Prometheus) is as described and builds, vets, and passes `go test
-race ./...` on `main`. The problems were in the evaluation and the claims
built on it.

| # | Finding | Severity | Evidence |
|---|---|---|---|
| 1 | The dataset behind every published number was gitignored. Nothing in README, METRICS, SYSTEM_DESIGN or the showcase site could be regenerated from a clone without a new, non-deterministic USD 3 API run. | High | `.gitignore` excluded `benchmarks/results/*.jsonl` and `*.csv` |
| 2 | The headline 91.6% cost reduction / 94% acceptance / 98.2% accuracy is not reproducible from any artifact. The preserved original trace replays to 84.8% at T=2.0 with the current (unchanged) cost model; the current trace gives 82.9%. README, SYSTEM_DESIGN, METRICS, PHASES, the Grafana legend and the live site all still showed 91.6% while RESULTS.md said it did not replicate. | High | `go run ./benchmarks/cmd/compare -input benchmarks/results/collected.pre_logprob.jsonl.gz -bootstrap=false` |
| 3 | The entropy router at T=2.0 escalates 40 of 519 requests and exactly one of them is a bad draft (TP 1, FP 39, FN 10). All 11 bad drafts have mean entropy 0.10 to 0.48 bits. A random router at the same rate matches its accuracy. The claimed 98.1% accuracy is 97.9% from the drafter alone plus one lucky catch. | High | `compare.csv` rows `entropy_router,2.0000` and `random_matched_rate` |
| 4 | The judge-based dataset cannot evaluate routers: 11 positives in 519, the judge (gpt-4.1) scores against gpt-4.1's own answer so the heavyweight is correct by construction, and 4 of the 11 negatives are 1024-token truncations rather than mistakes. Self-authored prompts, not a public benchmark. | High | `docs/RESULTS.md` section 1 |
| 5 | Cost model ignored input tokens entirely (`DrafterInputPer1M` and `HeavyInputPer1M` were defined but never used). | Medium | `benchmarks/internal/sweep/cost.go` at `001889b` |
| 6 | Threshold ceiling: top-5 renormalized entropy caps at log2(5) = 2.32 bits, so the T=2.25 and T=2.50 sweep rows are degenerate (0.8% and 0.0% escalation) and T=2.0 sits near the ceiling. | Medium | `internal/entropy/entropy.go`, max observed per-token entropy 2.3192 |
| 7 | Gateway buffers the full draft before responding; nothing streams to the client while the router decides. Measured drafter and heavyweight round trips are similar at ~290 output tokens (p50 3.6 s vs 3.7 s), so the README's "2-5x higher time-to-first-token" latency story was unsupported by the project's own data. | Medium | `internal/router/router.go`, `compare.json` latency fields |
| 8 | No failure handling: a drafter 5xx or timeout failed the request even though a healthy heavyweight was configured; a heavyweight failure after escalation failed it even though the drafter had answered. | Medium | `internal/gateway/handler.go` at `001889b` |
| 9 | RESULTS.md's reproduction command did not reproduce its own table (default confidence thresholds stopped at -8.0; the table went to -40). `docs/RESUME.md` is referenced by PHASES and CLAUDE.md but does not exist. The "P99 109ms" README figure had no committed command or config (`config.loadtest.yaml` was gitignored). | Low | `benchmarks/cmd/compare/main.go` defaults, `docs/PHASES.md` |
| 10 | Speculative execution's "~30% trigger, ~40% cancel" and the cache's "> 15% hit rate" in SYSTEM_DESIGN were never measured. | Low | no data in repo |
| 11 | The load-test mock's "uncertain" tokens had 1.58 bits of entropy (three alternatives), below T=2.0, so the mixed scenario never produced an escalation and the published p99 covered the accept path only. | Low | `benchmarks/cmd/loadtest/mock_server.go` at `001889b` |

## 2. Options considered and ranking

Effort is wall-clock hours for this pass. Keywords are the general technical
terms a host searching a resume database would match.

| Rank | Option | Effort | Keywords earned | Decision |
|---|---|---|---|---|
| 1 | Public benchmark subsets with answer keys (GSM8K, MMLU), exact-match grading of both models, bootstrap CIs | 3 h, < USD 2 | benchmark evaluation, statistical significance, ground-truth labeling, LLM-as-judge bias, reproducibility | Done |
| 2 | Stronger baselines: rate-matched random, always-drafter floor, oracle ceiling, mean-entropy variant; threshold-free AUROC | 1.5 h, USD 0 | baseline comparison, ROC/AUROC, ablation, calibration, confidence intervals | Done |
| 3 | Failure handling: drafter and heavyweight fallbacks, tests, flaky load-test scenario | 2 h, USD 0 | fault tolerance, graceful degradation, timeouts, fault injection, SLO | Done |
| 4 | Cost model with input tokens, USD per 1k requests, p50/p99 from recorded calls | 1 h, USD 0 | cost modeling, unit economics, latency percentiles, tail latency | Done |
| 5 | Commit frozen traces, make every number a replay | 0.5 h | reproducibility, data versioning | Done |
| 6 | Rerun the logprob artifact analysis on an open model via Ollama (provider-neutral `outside_top_k` flag, drafter-only mode) | 1 h, USD 0 | OpenAI-compatible APIs, logprobs, open-weight models, data quality | Done, 200 prompts |
| 7 | Docs and showcase site consistency, retire unreproducible numbers | 1 h | technical writing, honest reporting | Done |
| 8 | Gemini provider (cheap and strong Gemini models) | 2 h | multi-provider, vendor abstraction | Skipped: no `GEMINI_API_KEY` or `GOOGLE_API_KEY` configured, as instructed. The client is already OpenAI-compatible, so Gemini's OpenAI-compatible endpoint would be a config change plus logprob support verification. |
| 9 | Load test re-run with p50/p99 under the new fallback paths | 0.5 h | load testing, throughput, p99 | Done, see section 4 |
| 10 | Streaming the draft to the client while deciding (true TTFT win) | 4 h+ | streaming, backpressure | Not attempted: changes the product contract (a served draft cannot be retracted), needs a design decision |

## 3. What changed

Commits, oldest first (`git log --oneline 001889b..HEAD`):

1. **Frozen traces committed, gz-aware loader.** `collected.jsonl.gz` and the original `collected.pre_logprob.jsonl.gz` are in the repo; every sweep replays offline.
2. **Cost model prices input tokens; new baselines; AUROC; latency and USD/1k.** Policies now return the escalation point so serial latency can be estimated. `-output-only-pricing` reproduces the old model.
3. **Exact-match grading and public benchmark fetcher.** Prompts carry an answer key; both models are graded. `fetchpublic` samples GSM8K and MMLU with a fixed seed. Tokens record `outside_top_k`. Collector supports any OpenAI-compatible base URL and a drafter-only mode.
4. **Gateway graceful degradation.** Drafter failure routes to the heavyweight; heavyweight failure after escalation re-runs the drafter; both configurable, both tested (9 new handler tests, race-clean).
5. **Load test flaky scenario** with injected drafter 503s and stalls and per-decision tallies; `config.loadtest.yaml` tracked.
6. **GSM8K and MMLU traces and comparisons committed**, mean-entropy router variant, fraction-above-1-bit score.
7. **Docs rewritten** (README, RESULTS, SYSTEM_DESIGN, METRICS, PHASES), showcase site numbers replaced, Grafana legend fixed, tradeoff plots for all three datasets.

## 4. Before and after

### 4.1 Headline numbers

| Claim | Before (README on `main`) | After (this branch) | Command |
|---|---|---|---|
| Cost reduction at T=2.0 | 91.6%, no interval, no committed data | 82.9%, 95% CI 79.6% to 86.0%, replayed from committed trace, input and output tokens priced | `go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz -output benchmarks/results/compare.csv` |
| Draft acceptance | 94% | 92.3% (479 of 519) | same |
| Accuracy | 98.2% "draft path" | 98.1% served accuracy, CI 96.7% to 99.0% | same |
| Baseline comparison | always-heavyweight (100% by definition) and a min-logprob threshold | plus rate-matched random (98.1% at 83.9%), always-drafter (97.9% at 92.6%), oracle (100% at 87.7%), mean-entropy router (97.9% at 81.9%) | same |
| Signal quality | not measured | AUROC of the production statistic: 0.68 (project set), 0.51 (GSM8K), 0.47 (MMLU) | same, plus the gsm8k and mmlu CSVs |
| Public benchmarks | none | GSM8K: drafter 92.7%, heavyweight 95.3%, router 93.0%, random 92.7%. MMLU: drafter 75.7%, heavyweight 89.7%, router 77.3%, random 76.7% | `compare_gsm8k.csv`, `compare_mmlu.csv` |
| Cost per 1k requests | not reported | USD 0.534 (router) vs 3.127 (all heavyweight) on the project set; 0.293 vs 1.866 GSM8K; 0.549 vs 2.215 MMLU | `compare*.json` |
| Upstream latency | "2-5x TTFT" claim, no data | serial-path estimate from recorded calls: p50 3.7 s, p99 19.7 s project set; drafter and heavyweight round trips comparable | `compare.json` |
| Failure handling | request fails on any provider error | drafter outage served by heavyweight, heavyweight outage served by re-run drafter, both flagged on headers and metrics | `go test ./internal/gateway/ -run Fallback` |

### 4.2 Mixed and negative results, stated plainly

- At the production threshold the entropy router does not beat random escalation on any dataset. On the project set random is slightly cheaper at equal accuracy; on GSM8K the router is 0.3 points better than random and 0.3 points better than always-drafter; on MMLU it is 0.6 points better than random with 14 points of headroom left on the table.
- The windowed peak statistic, which is what enables early exit and speculative execution, is at chance on both public benchmarks (AUROC 0.51 and 0.47). The trace mean is informative (0.76 and 0.63) but needs the whole draft, which removes the speculative-execution latency story. This is a real design tension, not a tuning problem.
- Including input tokens barely moved the cost numbers (prompts are short), so the original omission was a correctness bug with small effect.
- The confidence-threshold baseline's 61.5% ceiling from the -9999 sentinel is confirmed, and on MMLU that simple baseline reaches higher accuracy (81.7%) than either entropy router at its ceiling, because MMLU has enough bad drafts for a conservative router to pay off.
- Mean-entropy router vs windowed router on GSM8K (94.3% vs 93.0%) has overlapping intervals with 22 positives. Suggestive, not established.

### 4.3 Open model logprob artifact

Measured on 200 prompts (50 per category) collected drafter-only from a
local qwen2.5-coder:7b via Ollama, zero API spend (`compare_qwen.csv`,
`docs/RESULTS.md` section 8.1):

- The sampled-token-outside-top-5 event is far more common on the open model (7.2% of creative tokens, 100% of creative records) than on gpt-4.1-nano (0.49%, 65.5%), and it occurs in every category (84% of code records, 68% of reasoning, 10% of factual).
- Ollama reports the real logprob for those tokens (median -4.42, minimum -12.47) instead of a -9999 sentinel. The min-logprob baseline consequently has no hard ceiling on the open model: its escalation rate falls from 61.5% at -5 to 0.0% at -15, where the OpenAI drafter is pinned at 18.7%. The "ceiling" in the earlier write-up is an API encoding artifact, which the earlier write-up guessed at and this run confirms.
- The entropy router at T=2.0 would escalate 24.5% of this model's drafts versus 7.7% for gpt-4.1-nano. Thresholds do not transfer between drafters.
- Limitation: no heavyweight or judge was run, so there is no accuracy for this model, and it is a different model family at default temperature. The flag itself is reliable: 2 of 1,788 flagged tokens are byte-split Unicode fragments mis-detected by string match, the rest have logprob below -1.

### 4.4 Gateway under load with injected failures

Gateway built from this branch against the mock upstream (20-token SSE
responses, 5 ms per chunk), speculative execution on, cache off, 50 req/s
for 30 s per scenario, drafter timeout 2 s (`config.loadtest.yaml`):

| Scenario | Requests | Success | p50 | p95 | p99 | Routing decisions |
|---|---|---|---|---|---|---|
| confident (all drafts accepted) | 1500 at 50 req/s | 100% | 105.9 ms | 106.9 ms | 107.4 ms | accept 1500 |
| mixed (20% of drafts uncertain) | 1500 at 50 req/s | 100% | 106.3 ms | 108.4 ms | 109.8 ms | accept 1188, escalate 312 |
| flaky (20% drafter 503s, 5% drafter stalls, 2 s drafter timeout) | 1500 at 50 req/s | 100%, zero 5xx | 107.1 ms | 111.0 ms | 2107.9 ms | accept 895, escalate 249, fallback_heavyweight 356 |

The mock responds in about 100 ms (20 chunks at 5 ms), so p50 is upstream time and gateway overhead is in the low milliseconds. In the flaky run every injected drafter failure was served by the heavyweight; the p99 is the 2 s drafter timeout plus one heavyweight call, which is the cost of a stall rather than an error. The heavyweight is never faulted in this scenario, so `fallback_draft` does not appear; its path is covered by unit tests. Before this pass the mixed scenario's uncertain tokens carried 1.58 bits of entropy, below T=2.0, so the earlier load tests never exercised escalation.

Before this pass the same mock's uncertain tokens never crossed T=2.0, so
the earlier "mixed" load test (and the README's 109 ms p99) measured the
accept path only. The 107 ms figure above replaces it and the escalation
path is now exercised at about 21% of requests.

## 5. Reproducing everything

```bash
git checkout enhance/evaluation-hardening
go test -race ./...

# Offline replays (seconds, no API key)
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz       -output benchmarks/results/compare.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k.jsonl.gz -output benchmarks/results/compare_gsm8k.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_mmlu.jsonl.gz  -output benchmarks/results/compare_mmlu.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_qwen2.5-coder-7b.jsonl.gz -output benchmarks/results/compare_qwen.csv -bootstrap=false
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.pre_logprob.jsonl.gz -output /tmp/old.csv -bootstrap=false   # the 84.8% figure
python benchmarks/plot_tradeoff.py

# Load test (two terminals, no API key)
OPENAI_API_KEY=mock ./draft-thinker --config config.loadtest.yaml
go run ./benchmarks/cmd/loadtest -scenario confident -rate 50 -duration 30s
go run ./benchmarks/cmd/loadtest -scenario mixed     -rate 50 -duration 30s
go run ./benchmarks/cmd/loadtest -scenario flaky     -rate 50 -duration 30s

# Re-collect (API spend, non-deterministic)
go run ./benchmarks/cmd/fetchpublic -dataset gsm8k -n 300 -seed 42
go run ./benchmarks/cmd/fetchpublic -dataset mmlu  -n 300 -seed 42
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/gsm8k_300.json -output benchmarks/results/collected_gsm8k.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/mmlu_300.json  -output benchmarks/results/collected_mmlu.jsonl
go run ./benchmarks/cmd/collect -drafter-only -drafter-base-url http://localhost:11434/v1 -drafter-model qwen2.5-coder:7b -drafter-key-env NONE -limit 50 -max-tokens 512 -output benchmarks/results/collected_qwen2.5-coder-7b.jsonl
```

## 6. Keywords earned

benchmark evaluation with bootstrap confidence intervals, baseline and ablation design (random, oracle, floor, ceiling), ROC/AUROC analysis, cost modeling and unit economics (USD per 1k requests), tail latency (p50/p99), fault tolerance and graceful degradation, fault injection load testing, reproducible data pipelines.

## 7. Google-scale stories (three sentences each)

**Public benchmarks with answer keys.** Serving teams at Google scale cannot
accept a routing policy whose only evidence is a self-authored prompt set
scored by the same model it is meant to replace, because the first
distribution shift exposes it. Moving to GSM8K and MMLU with exact-match
grading of both models turned a definitional 100% ceiling into a measured
95.3% and 89.7%, and exposed that the shipped statistic is at chance. That
is the same discipline as holding out traffic slices and labeled sets before
a model launch in Search or Ads ranking.

**Honest baselines and AUROC.** Any router that spends 8% of traffic on the
expensive path should be forced to beat a coin flip that spends the same
8%, and here it did not. At Google volume a cost-reduction headline without
that control would misallocate real serving capacity, so the rate-matched
random and oracle bounds are the experiment design, not a nicety. The AUROC
table separates "is the signal informative" from "is the threshold right",
which is how you decide whether to tune or to redesign.

**Graceful degradation.** A gateway in front of two model providers is a
dependency fan-out, and at SRE scale the question is never whether an
upstream fails but what the user sees when it does. Routing drafter failures
to the heavyweight and heavyweight failures back to a re-run drafter turns a
provider incident into a change in routing mix on a dashboard instead of an
error-budget burn. The fault-injection load test is the runbook check that
the fallback actually engages under load rather than only in a unit test.

**Cost per 1k requests and tail latency from recorded calls.** Capacity and
budget planning at scale run on unit costs and tail percentiles, not on
percentage headlines. Pricing input tokens, reporting USD per 1k requests
per method, and deriving serial-path p50/p99 from the recorded calls made
the tradeoff legible in the units a planning review uses. It also surfaced
that the drafter is not faster end to end at this output length, which
changes what the latency story can honestly claim.

**Reproducible evaluation artifacts.** Engineering Productivity work is
largely about making the next person able to regenerate your number in
seconds, so the trace files are committed, every table has a command, and a
retired figure is explained rather than deleted. The off-by-one found in the
rate-matching helper while writing its test is the small concrete example of
why the tooling needs tests as much as the product does.

## 8. What I would do next

1. Replace the windowed peak with a running mean gated on a minimum token
   count, calibrate it on GSM8K and MMLU, and measure what that costs the
   speculative-execution latency win.
2. Collect a second independent GSM8K and MMLU sample to check that the
   mean-entropy advantage replicates outside one draw.
3. Stream accepted drafts to the client as they arrive once the router is
   confident enough to commit, so the time-to-first-token story becomes
   measurable instead of asserted.
4. Measure speculative trigger and cancellation rates and cache hit rate
   against replayed real traffic; both are still unmeasured.
