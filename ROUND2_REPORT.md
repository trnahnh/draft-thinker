# Round 2 Report: replication, a learned router, and measuring what was claimed

Branch: `round2/draft-thinker`, built on `enhance/evaluation-hardening`
(round 1, `05a7ffc`). `main` is untouched. Every number here is produced by
a command in section 5 from data committed on the branch. API spend for
this round, from the token counts in the traces at list prices: USD 3.55
for model calls (GSM8K second sample 1.04, ARC-Challenge 0.79, MMLU second
sample 1.16, gpt-4.1 heavyweight for the open-model run 0.56) plus about
USD 0.03 of embeddings for the cache replays. No single run exceeded USD
1.2, so no approval stop was triggered; the USD 10 cap was not approached.

## 1. Audit delta: what round 1 left open and what round 2 found

| # | Round 1 state | Round 2 finding | Evidence |
|---|---|---|---|
| 1 | Every router result rested on one seeded sample per benchmark (300 prompts). | Replicated on disjoint second samples (500 each, seed 7, round 1 rows excluded) and on ARC-Challenge: the production statistic is at chance on all five answer-key samples (AUROC 0.45 to 0.57), the router at T=2.0 is 0.2 to 1.0 accuracy points below rate-matched random on all three new sets, the trace mean is informative on all five (0.70 to 0.81). | `compare_gsm8k_r2.json`, `compare_mmlu_r2.json`, `compare_arc_r2.json` |
| 2 | The mean-entropy advantage over the windowed router was "suggestive, not established" (22 positives). | Established within the limits of 500-prompt samples: +1.8, +3.4 and +0.4 points over the windowed router on GSM8K, MMLU and ARC, with intervals that overlap. Over random: +1.4, +2.4, +0.2. | same |
| 3 | No attempt at a better router. | A learned whole-trace router (22 trace features, logistic and gradient boosting, trained on round 1, tested on round 2) beats the production statistic by 0.23 to 0.27 AUROC with disjoint intervals and beats random by 0.8 to 3.0 points at matched escalation. It is indistinguishable from mean entropy alone at this training size (600 records). Prefix-only variants are at chance. | `benchmarks/models/`, section 4.2 |
| 4 | Speculative execution's trigger and cancel rates were "not measured; counters exist". | Measured by replaying every trace through the production executor: 70 to 89% of requests trigger, 93 to 99% of those cancel, median head start 26 to 37 ms, and most escalations get no head start at all. The soft trigger is per token, not per window, which the design doc had wrong. The Phase 4 target of under 10% waste is not met at any multiplier. | `speculative_*.json` |
| 5 | Cache hit rate "not measured". | Measured by replaying traces through the production store with the real embedding client: 0% hits on the project set and GSM8K, 1.0% on MMLU (one of three hits served a wrong answer to a templated near-duplicate), 100% on exact repeats. Lookup p50 about 200 ms, not the documented 10 to 50 ms. At 0.90 the MMLU replay would serve 19 wrong answers in 30 hits. | `cache_replay_*.json` |
| 6 | Open model run was drafter-only, no accuracy. | qwen2.5-coder:7b as drafter with gpt-4.1 heavyweight on GSM8K, exact-match graded: drafter 83.7%, heavyweight 97.0%, router at T=2.0 87.3% (escalating 28.0%), random at that rate 88.0%. Min-logprob sweep runs its full range (no sentinel ceiling), AUROC 0.59; window peak 0.52; mean 0.73. | `compare_gsm8k_qwen.json` |
| 7 | The design doc described the speculative trigger as "first 10 tokens: windowed entropy > 0.8T". | The code tests each token individually against 0.8T and checks the hard threshold first, so a token above T escalates without speculation. Docs corrected. | `internal/speculative/executor.go` |
| 8 | Phase 4 and 5 exit criteria ("< 10% waste", "< 50 ms cached response") were marked complete. | Both measured and not met; marked as such with the numbers. | `docs/PHASES.md` |
| 9 | Gemini provider pending a key. | Still no `GEMINI_API_KEY` or `GOOGLE_API_KEY` in the environment; skipped as instructed. | shell check at start of round |

## 2. Options considered and ranking

Effort is wall-clock hours in this round. Keywords are the general terms a
host searching a resume database would match.

| Rank | Option | Effort | Keywords earned | Decision |
|---|---|---|---|---|
| 1 | Replication on disjoint resamples plus a third benchmark (ARC-Challenge), with exclusion of round 1 rows | 2 h, USD 3 | replication, held-out evaluation, sampling, benchmark construction, confidence intervals | Done |
| 2 | Learned router: Go feature export, Python training, JSON export, Go evaluator with parity test, isotonic calibration, served-error operating points, prefix ablation, pooled cross-validation | 4 h, USD 0 | feature engineering, logistic regression, gradient boosting, calibration, conformal risk control, train/test discipline, model export | Done, negative on "better than mean entropy", positive on "better than production" |
| 3 | Measure speculative execution by replaying traces through the real executor with synthetic logprobs that reproduce recorded entropy | 1.5 h, USD 0 | replay testing, offline evaluation of online systems, waste accounting | Done, strongly negative for the feature |
| 4 | Measure the semantic cache by replaying traces through the real store with an in-memory index | 1.5 h, under USD 0.05 | semantic caching, embeddings, false-positive analysis, latency measurement | Done, negative on hit rate, positive on threshold choice |
| 5 | Open model with heavyweight and exact grading | 1 h wall-clock (65 min of local GPU), USD 0.56 | open-weight models, OpenAI-compatible serving, cross-model transfer | Done |
| 6 | Streaming design note | 1 h | streaming, API contract design, decision records | Done, `docs/STREAMING_DESIGN.md` |
| 7 | Gemini provider | 2 h | multi-provider | Skipped, no key configured |
| 8 | Retrain the learned router per drafter (qwen) and per dataset calibration | 1 h | per-route calibration | Not done; the calibration-transfer failure in 4.4 says it is the next step |
| 9 | Measure what a cancelled speculative stream actually bills | 1 h, small spend | billing semantics | Not done; needs live calls with timed cancels |

## 3. What changed

Commits, oldest first (`git log --oneline 05a7ffc..HEAD`):

1. **Fetcher and grading.** `fetchpublic` gains ARC-Challenge, `-exclude` (disjoint resampling by source row) and `-id-prefix`; choice grading accepts A to E.
2. **Learned router tooling.** `benchmarks/cmd/features` (feature export), `benchmarks/train_router.py` (training, export, calibration, operating points, parity rows), `sweep/learned.go` (Go evaluator), parity and unit tests, `compare -models`.
3. **Replay tools.** `benchmarks/cmd/replayspec` (speculative executor replay) and `benchmarks/cmd/replaycache` (cache store replay with retries).
4. **Data.** Three round 2 traces, the open-model GSM8K trace with heavyweight, feature CSVs, five exported models with reports, speculative and cache replay summaries, comparison CSV/JSON for every trace, tradeoff plots with bootstrap error bars.
5. **Docs.** `docs/RESULTS.md` sections 9 to 14, `docs/STREAMING_DESIGN.md`, corrections to `SYSTEM_DESIGN.md`, `PHASES.md`, `METRICS.md`, `README.md`, the showcase site's speculative and cache cards, this report.

No gateway code changed in this round. The findings argue for a
configuration change (speculative execution off) rather than a code change,
and the router redesign they point to is a product decision covered in the
streaming note.

## 4. Before and after

### 4.1 Replication

| Claim after round 1 | Round 2 | Replicated? |
|---|---|---|
| Windowed statistic at chance on GSM8K (AUROC 0.51) and MMLU (0.47) | 0.57 (0.48 to 0.66) on GSM8K, 0.45 (0.39 to 0.50) on MMLU, 0.52 (0.42 to 0.62) on ARC | Yes |
| Trace mean informative (0.76 GSM8K, 0.63 MMLU) | 0.81 (0.73 to 0.87), 0.70 (0.65 to 0.74), 0.72 (0.65 to 0.80) on ARC | Yes |
| Router at T=2.0 within 0.6 points of random | 0.4, 1.0 and 0.2 points below random | Yes |
| Drafter alone within 1 point of the router | 0.4, 1.4 and 0.4 points below the router | Yes |
| Heavyweight accuracy 95.3% GSM8K, 89.7% MMLU | 95.8%, 90.8%; ARC 96.4% | Consistent |

### 4.2 Learned router, held out

| Dataset | Production router (T=2.0) | Random, same rate | Mean-entropy, same rate | Learned logistic, same rate | AUROC: production / mean / learned |
|---|---|---|---|---|---|
| GSM8K r2 | 90.4% | 90.8% | 92.2% | 92.4% (90.0 to 94.6) | 0.569 / 0.805 / 0.808 |
| ARC | 93.0% | 93.2% | 93.4% | 94.0% (91.8 to 96.0) | 0.519 / 0.718 / 0.748 |
| MMLU r2 | 78.2% | 79.2% | 81.6% | 82.2% (79.0 to 85.6) | 0.445 / 0.695 / 0.715 |
| Project set (judge) | 98.1% | 98.1% | 97.9% | 98.3% (97.1 to 99.2) | 0.684 / 0.515 / 0.741 |

Stated plainly: the learned router beats random on every answer-key set,
by less than the width of the interval; it beats the shipped statistic
decisively on AUROC; it does not beat the trace mean. With 2,100 pooled
records and cross-validation it reaches 0.794 (0.767 to 0.820) against
0.719 (0.688 to 0.748) for the mean, so more labels would likely separate
them, but that is a projection, not a held-out result.

### 4.3 Prefix routers (what early exit could use)

Held-out AUROC with features from the first 20 tokens: 0.488, 0.483,
0.586. First 50: 0.591, 0.518, 0.622. At chance or close to it. The
information that predicts a wrong answer arrives with the answer.

### 4.4 Calibrated operating point transfer

The 8% served-error point chosen on round 1 folds gave 6.4% error on GSM8K
r2 (met), 3.8% on ARC by escalating 66.8% of requests (met, wastefully),
and 11.0% on MMLU r2 (missed; the heavyweight alone is 9.2% wrong there).
The 10% point gave 7.4%, 5.6% and 14.6%. Risk control needs calibration
data from the traffic it governs.

### 4.5 Speculative execution (replay through the production executor)

| Trace | Trigger rate | Cancel ratio | Escalations with a head start | Head start p50 | Cancelled calls per 1k requests |
|---|---|---|---|---|---|
| Project set | 70.5% | 96.4% | 13 of 40 | 36 ms | 680 |
| GSM8K r1 / r2 | 87.7% / 88.8% | 98.9% / 98.6% | 3 of 16 / 6 of 34 | 26 / 37 ms | 867 / 876 |
| MMLU r1 / r2 | 86.0% / 86.4% | 93.4% / 92.6% | 17 of 56 / 32 of 95 | 27 / 29 ms | 803 / 800 |
| ARC | 88.8% | 96.4% | 16 of 66 | 30 ms | 856 |

Before: "not measured", design doc said waste would be small because
escalations are 7.7%. After: the trigger is per token, almost every draft
has one token above 1.6 bits, and if cancelled calls billed to completion
they would cost 80 to 88% of the all-heavyweight bill. Recommendation in
the docs: `speculative.enabled: false`.

### 4.6 Semantic cache (replay through the production store)

| Trace | Hits at 0.95 | Wrong-answer hits | Nearest-neighbour max | Lookup p50 / p99 |
|---|---|---|---|---|
| Project set (519) | 0 | 0 | 0.878 | 197 / 374 ms |
| GSM8K (300) | 0 | 0 | 0.763 | 199 / 406 ms |
| MMLU (300) | 3 | 1 | 1.000 (exact duplicates exist in MMLU) | 201 / 360 ms |

Before: "not measured", "< 50 ms", "~10 to 15 ms overhead". After: the
numbers above; the 0.95 threshold is justified by data (0.90 would serve
19 wrong answers in 30 MMLU hits); exact repeats hit 479 of 479.

### 4.7 Open model with grading

| Method (GSM8K, 300 prompts) | Escalation | Served accuracy |
|---|---|---|
| qwen2.5-coder:7b alone | 0% | 83.7% (79.3 to 87.7) |
| entropy router T=2.0 | 28.0% | 87.3% (83.3 to 91.0) |
| random, same rate | 28.7% | 88.0% (84.3 to 91.7) |
| mean-entropy router, same rate | 28.0% | 90.3% (86.7 to 93.7) |
| gpt-4.1 alone | 100% | 97.0% (95.0 to 98.7) |

The confidence-threshold sweep runs from 100% escalation at -1 to 0% at
-15 with no sentinel floor, closing the round 1 artifact question. Cost
columns for this trace price the local drafter at gpt-4.1-nano rates as a
stand-in and are labelled as such.

## 5. Reproducing everything

```bash
git checkout round2/draft-thinker
go test -race ./...

# Offline replays (no API key)
M=benchmarks/models/router_full_logistic.json,benchmarks/models/router_full_gbt.json,benchmarks/models/router_prefix20_logistic.json,benchmarks/models/router_prefix50_logistic.json
for f in gsm8k_r2 mmlu_r2 arc_r2; do go run ./benchmarks/cmd/compare -input benchmarks/results/collected_$f.jsonl.gz -output benchmarks/results/compare_$f.csv -models $M; done
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz -output benchmarks/results/compare.csv -models $M
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k_qwen2.5-coder-7b.jsonl.gz -output benchmarks/results/compare_gsm8k_qwen.csv
for f in collected collected_gsm8k collected_mmlu collected_gsm8k_r2 collected_arc_r2 collected_mmlu_r2 collected_gsm8k_qwen2.5-coder-7b; do go run ./benchmarks/cmd/replayspec -input benchmarks/results/$f.jsonl.gz; done
python benchmarks/plot_tradeoff.py benchmarks/results/compare_gsm8k_r2.csv benchmarks/results/tradeoff_gsm8k_r2.png "GSM8K replication (n=500, held out)"

# Learned router training (Python offline; parity test in Go)
go run ./benchmarks/cmd/features -input benchmarks/results/collected_gsm8k.jsonl.gz,benchmarks/results/collected_mmlu.jsonl.gz -output benchmarks/results/features_train.csv
python benchmarks/train_router.py --train benchmarks/results/features_train.csv --name full
go test ./benchmarks/internal/sweep/ -run Learned

# Cache replay (embedding calls, about one cent per thousand prompts)
OPENAI_API_KEY=... go run ./benchmarks/cmd/replaycache -input benchmarks/results/collected.jsonl.gz -repeat
OPENAI_API_KEY=... go run ./benchmarks/cmd/replaycache -input benchmarks/results/collected_mmlu.jsonl.gz

# Re-collect (API spend, non-deterministic): see docs/RESULTS.md section 14
```

## 6. Keywords earned

replication and held-out evaluation, feature engineering from token-level
traces, logistic regression and gradient boosting with cross-language model
export, probability calibration and risk-controlled operating points,
offline replay of online components, waste accounting for speculative
systems, semantic cache false-positive analysis, open-weight model
evaluation.

## 7. Google-scale stories (three sentences each)

**Replication before belief.** A launch review at Google scale asks whether
the metric moved because of the change or because of the sample, and the
only answer that survives is a second, independent draw. Round 2 drew new
samples of both benchmarks and a third benchmark, and every negative
finding from round 1 came back with the same sign and tighter intervals.
That is cheaper than one wrong launch and it is the habit the exercise was
meant to build.

**A learned router under a real train/test split.** Training on one draw
and testing on another is the minimum discipline for any ranking or
routing model that will see fresh traffic, and it is what separated "GBT
AUROC 0.955 in sample" from "0.79 held out" here. The learned router did
beat the shipped heuristic and random, by a margin a 500-prompt sample
cannot certify, and it did not beat a one-line rule. Reporting that
honestly is what makes the next iteration (more labels, per-route
calibration) a defensible ask rather than a hope.

**Measuring a feature by replaying it through its own code.** Speculative
execution had counters nobody read and a design doc that described a
different trigger than the one in the code. Feeding recorded traces through
the production executor showed it fires on most requests, cancels nearly
all of them, and buys tens of milliseconds, which is the kind of
cost-to-benefit mismatch that capacity planning at scale would catch as a
runaway heavyweight QPS. The fix is a config flag, and the tool now exists
to re-check it after any change.

**Caches that can lie.** A semantic cache is a correctness risk disguised as
a latency feature, and at Google scale a 1% wrong-answer rate on a hot
path is an incident. The replay showed that the 0.95 threshold hits almost
nothing on unique traffic and that relaxing it to 0.90 would serve wrong
answers on two thirds of its hits on templated prompts. The measurement
turned "conservative by design" into "conservative because here is what
happens otherwise".

**Calibration does not travel.** The served-error guarantee held on the
distribution it was calibrated on and failed on both easier and harder
ones, which is exchangeability failing in the plainest way. Any serving
system that promises an error budget has to calibrate on the traffic slice
it governs and re-calibrate on drift. That lesson generalises well beyond
LLM routing.

## 8. What remains unverified

1. **What a cancelled speculative stream actually costs.** The replay gives
   an upper bound (billing to completion). The true figure needs live calls
   with timed cancellation against the provider.
2. **Whether more labels make the learned router beat mean entropy out of
   sample.** The pooled cross-validation says yes; a held-out test on a
   fourth draw would say for sure.
3. **Per-drafter retraining.** The learned router was not retrained for the
   open model; its thresholds and features assume gpt-4.1-nano.
4. **Cache hit rate on real traffic.** Benchmark prompts are unique by
   construction; production traffic with genuine repeats would hit more,
   and only a production log can say how much.
5. **Streaming time-to-first-token.** The design note quantifies what is
   known; the collector does not record chunk arrival times, so the drafter's
   time to k tokens is not measurable from the committed traces.
6. **Gemini logprob support.** Not tested; no key.
