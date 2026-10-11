# Results: Entropy Router vs. Baselines

Every number in this document is produced by a command listed in section 14
from trace files committed under `benchmarks/results/`. Nothing is estimated
or carried over from earlier documentation. Where a result is negative for
the project's thesis, it is reported as such.

**Headline, stated carefully.** On the project's own 519-prompt benchmark the
windowed entropy router at the calibrated threshold T=2.0 cuts measured
inference cost by 82.9% (bootstrap 95% CI 79.6% to 86.0%) at 98.1% served
accuracy (CI 96.7% to 99.0%). However, a router that escalates the same
fraction of requests at random reaches the same 98.1% accuracy at 83.9% cost
reduction, and sending every request to the drafter alone reaches 97.9% at
92.6%. On two public benchmarks with answer keys (GSM8K and MMLU) the
windowed statistic has AUROC 0.51 and 0.47 as a detector of wrong drafts,
which is chance. The cost savings are real and reproducible; they come from
the drafter being cheap and usually right, not from the entropy signal
selecting which requests to escalate. Trace-level statistics (mean entropy,
minimum logprob) do carry signal (AUROC 0.63 to 0.76), which points to a
better router design discussed in section 7.

**Round 2 (sections 9 to 13).** Every one of those findings replicated on
new, disjoint samples of GSM8K and MMLU (500 prompts each) and on
ARC-Challenge: the windowed statistic is at chance on all five answer-key
samples, the router at T=2.0 is 0.2 to 1.0 accuracy points below
rate-matched random on all three new sets, and the trace mean is
informative on all of them. A learned whole-trace router trained on round 1
and tested on round 2 beats the production statistic clearly, beats random
by 0.8 to 3.0 points, and is indistinguishable from the one-line
mean-entropy rule. Replaying the recorded traces through the production
speculative executor and cache showed that speculative execution fires on
70 to 89% of requests and cancels 93 to 99% of those calls, and that the
semantic cache hits 0 to 1% of unique benchmark prompts while its lookup
costs about 200 ms per request.

---

## 1. Datasets

Round 1 used three datasets; round 2 added four more traces. All are committed so the sweeps replay offline:

| Dataset | Prompts | Labels | Drafter wrong | Heavyweight wrong | Trace file |
|---|---|---|---|---|---|
| Project set (4 categories) | 519 valid of 520 | LLM-as-judge (gpt-4.1) vs. heavyweight reference | 11 (2.1%) | n/a (reference answer) | `collected.jsonl.gz` |
| GSM8K test, seeded sample | 300 | exact numeric match to answer key | 22 (7.3%) | 14 (4.7%) | `collected_gsm8k.jsonl.gz` |
| MMLU test, seeded sample | 300 | exact A to D match to answer key | 73 (24.3%) | 31 (10.3%) | `collected_mmlu.jsonl.gz` |
| GSM8K, second disjoint sample (round 2) | 500 | exact numeric match | 50 (10.0%) | 21 (4.2%) | `collected_gsm8k_r2.jsonl.gz` |
| MMLU, second disjoint sample (round 2) | 500 | exact A to D match | 116 (23.2%) | 46 (9.2%) | `collected_mmlu_r2.jsonl.gz` |
| ARC-Challenge test, seeded sample (round 2) | 500 | exact A to E match | 37 (7.4%) | 18 (3.6%) | `collected_arc_r2.jsonl.gz` |
| GSM8K sample 1 with qwen2.5-coder:7b drafter (round 2) | 300 | exact numeric match | 49 (16.3%) | 9 (3.0%) | `collected_gsm8k_qwen2.5-coder-7b.jsonl.gz` |

**Project set.** 150 simple factual, 150 multi-step reasoning, 110 code
generation, 110 ambiguous/creative prompts (`benchmarks/testdata/prompts.json`).
One prompt failed collection with a transient read timeout. The judge
(`benchmarks/internal/collect/judge.go`) scores the draft 1 to 5 against the
heavyweight response; score 3 or above is acceptable. Two caveats: the judge
and the reference model are both gpt-4.1, so the heavyweight can never be
judged wrong, and 4 of the 11 unacceptable drafts hit the 1024-token cap and
were marked wrong for being truncated rather than for being mistaken.

**GSM8K and MMLU.** Fetched by `benchmarks/cmd/fetchpublic` with seed 42
(300 of 1319 GSM8K test rows; 60 pages of 5 rows from the 14042-row MMLU test
split, 35 subjects). Prompts ask the model to reason then write a final
answer line; both models are graded against the answer key, so
always-heavyweight accuracy is measured rather than definitional. 9 of 300
MMLU drafts and 2 heavyweight responses produced no parseable letter and
count as wrong.

Drafter: gpt-4.1-nano with top-5 logprobs. Heavyweight: gpt-4.1. Collection
cost for GSM8K plus MMLU was under USD 2 at list prices.

## 2. Methods compared

All methods are `sweep.EscalationPolicy` functions in
`benchmarks/internal/sweep/policy.go`, replayed against the same recorded
traces. No API calls happen during a sweep.

| Method | Decision rule | Needs full draft? |
|---|---|---|
| `entropy_router` | Production algorithm: 10-token sliding window over per-token Shannon entropy (bits, top-5 renormalized); escalate when a token in the first 10 exceeds T or the window average exceeds T | No (early exit) |
| `mean_entropy_router` | Escalate when the trace-wide mean entropy exceeds a threshold chosen so the escalation rate matches `entropy_router` at T=2.0 | Yes |
| `confidence_threshold` | Escalate when the minimum sampled-token logprob is below a threshold | Yes |
| `random_matched_rate` | Escalate a fixed fraction chosen by hashing the prompt ID, matched to `entropy_router` at T=2.0 | n/a |
| `always_drafter` | Never escalate (cost floor) | n/a |
| `always_heavyweight` | Always escalate (cost ceiling) | n/a |
| `oracle` | Escalate exactly the wrong drafts (unattainable upper bound) | n/a |

**Cost model** (`benchmarks/internal/sweep/cost.go`): gpt-4.1-nano at USD
0.20 / 0.80 per 1M input / output tokens, gpt-4.1 at USD 2.50 / 10.00. Both
input and output tokens are priced; prompt tokens come from provider usage
where recorded and a chars/4 estimate on the older project-set traces. The
earlier calibration ignored input tokens entirely. `-output-only-pricing`
reproduces that model; on the project set it moves cost reduction at T=2.0
by less than 0.1 point because prompts are short. Routers pay the full
drafter output even when escalating (the stream is aborted, but tokens
already generated are billed).

**Accuracy** is the fraction of served responses that were correct. With
the LLM judge an escalated request is correct by construction. With answer
keys, an escalation to a heavyweight that is also wrong counts as wrong.

**Latency** is estimated from the per-call latencies recorded during
collection for the serial path: an accepted draft costs the drafter call;
an escalation costs the fraction of the drafter call before the decision
plus the heavyweight call. These are OpenAI round trips from a residential
connection at 3-way concurrency, not gateway overhead.

**Intervals** are percentile bootstrap, 2000 resamples of records with
replacement, seed 42.

## 3. Project set (519 prompts, LLM-as-judge)

| Method | Threshold | Escalation | Accuracy (95% CI) | Cost reduction (95% CI) | USD / 1k requests |
|---|---|---|---|---|---|
| entropy_router | 2.00 | 7.7% | 98.1% (96.7 to 99.0) | 82.9% (79.6 to 86.0) | 0.534 |
| mean_entropy_router | 0.77 | 7.7% | 97.9% (96.5 to 99.0) | 81.9% (78.4 to 85.1) | 0.567 |
| confidence_threshold | -15.00 | 18.7% | 98.8% (97.9 to 99.6) | 61.7% (56.5 to 66.9) | 1.199 |
| random_matched_rate | 7.7% | 7.9% | 98.1% (96.7 to 99.0) | 83.9% (80.6 to 86.7) | 0.504 |
| always_drafter | n/a | 0.0% | 97.9% (96.5 to 99.0) | 92.6% (92.3 to 92.8) | 0.233 |
| always_heavyweight | n/a | 100.0% | 100.0% (definitional) | 0.0% | 3.127 |
| oracle | n/a | 2.1% | 100.0% | 87.7% (84.5 to 90.3) | 0.384 |

Confusion matrix for entropy_router at T=2.0: TP 1, FP 39, TN 469, FN 10.
The router escalated 40 requests and exactly one of them was a bad draft.
Random escalation at the same rate also caught one. All 11 bad drafts have
mean entropy between 0.10 and 0.48 bits; they are confident mistakes.

Full sweep: `benchmarks/results/compare.csv`. Per category:
`benchmarks/results/compare_by_category.csv`. At T=2.0 the router escalates
19.1% of creative prompts, 11.4% of reasoning, 1.8% of code and 0.0% of
factual prompts; 10 of the 11 bad drafts are in reasoning, where served
accuracy is 94.0%.

## 4. GSM8K (300 prompts, answer key)

The drafter solves 92.7% and the heavyweight 95.3%. 13 drafts are wrong where
the heavyweight is right, so the most any router can gain is 4.3 points.

| Method | Threshold | Escalation | Accuracy (95% CI) | Cost reduction (95% CI) | USD / 1k requests |
|---|---|---|---|---|---|
| entropy_router | 2.00 | 5.3% | 93.0% (90.0 to 95.7) | 84.3% (81.4 to 87.0) | 0.293 |
| mean_entropy_router | 0.40 | 5.3% | 94.3% (91.7 to 96.7) | 83.9% (80.1 to 86.9) | 0.301 |
| confidence_threshold | -10.00 | 5.0% | 93.7% (90.7 to 96.3) | 83.8% (79.9 to 87.2) | 0.303 |
| random_matched_rate | 5.3% | 5.3% | 92.7% (89.3 to 95.3) | 84.6% (81.4 to 87.3) | 0.288 |
| always_drafter | n/a | 0.0% | 92.7% (89.3 to 95.3) | 90.2% (89.7 to 90.6) | 0.183 |
| always_heavyweight | n/a | 100.0% | 95.3% (93.0 to 97.7) | 0.0% | 1.866 |
| oracle | n/a | 7.3% | 97.0% (95.0 to 98.7) | 82.5% (78.8 to 85.8) | 0.326 |

entropy_router at T=2.0 caught 2 of 22 wrong drafts (TP 2, FP 14, FN 20).
The mean-entropy variant caught 6 of 22 at the same escalation rate (TP 6,
FP 10, FN 16). The intervals overlap, so this is suggestive, not conclusive.

## 5. MMLU (300 prompts, answer key)

MMLU is the hardest set for the drafter (75.7%) and has the most headroom:
the heavyweight reaches 89.7%, and 50 drafts are wrong where the heavyweight
is right.

| Method | Threshold | Escalation | Accuracy (95% CI) | Cost reduction (95% CI) | USD / 1k requests |
|---|---|---|---|---|---|
| entropy_router | 2.00 | 18.7% | 77.3% (72.7 to 82.0) | 75.2% (70.4 to 79.9) | 0.549 |
| mean_entropy_router | 0.77 | 18.7% | 79.3% (74.7 to 83.7) | 71.9% (66.3 to 76.9) | 0.623 |
| confidence_threshold | -12.00 | 23.0% | 81.7% (77.0 to 86.0) | 62.6% (56.2 to 69.0) | 0.828 |
| random_matched_rate | 18.7% | 18.7% | 76.7% (72.0 to 81.3) | 74.8% (69.7 to 79.1) | 0.558 |
| always_drafter | n/a | 0.0% | 75.7% (71.0 to 80.3) | 92.7% (92.2 to 93.0) | 0.163 |
| always_heavyweight | n/a | 100.0% | 89.7% (86.0 to 93.0) | 0.0% | 2.215 |
| oracle | n/a | 24.3% | 92.3% (89.3 to 95.0) | 62.2% (55.8 to 68.8) | 0.837 |

Here the production router buys 1.6 points of accuracy over always-drafter
for 17.5 points of cost reduction, and random escalation buys 1.0 point for
the same spend. The oracle shows that 14 points were available at a 62% cost
reduction. The simple min-logprob baseline gets closer to that than either
entropy router does.

## 6. Signal quality independent of threshold (AUROC)

AUROC of each per-request score as a detector of wrong drafts. 0.5 is
chance. `entropy_window_peak` is the exact statistic the production router
thresholds (a record escalates at T iff its peak exceeds T).

| Score | Project set (11 pos.) | GSM8K (22 pos.) | MMLU (73 pos.) |
|---|---|---|---|
| entropy_window_peak | 0.684 (0.590 to 0.779) | 0.509 (0.357 to 0.649) | 0.474 (0.399 to 0.551) |
| mean_entropy | 0.515 (0.382 to 0.643) | 0.760 (0.650 to 0.858) | 0.626 (0.547 to 0.698) |
| frac_tokens_above_1bit | 0.527 (0.400 to 0.654) | 0.720 (0.597 to 0.830) | 0.624 (0.550 to 0.694) |
| neg_min_logprob | 0.714 (0.554 to 0.861) | 0.621 (0.493 to 0.742) | 0.682 (0.611 to 0.745) |

Reading: the windowed peak is the only score that is informative on the
project set and the only one that is uninformative on both public sets.
The project set's positives are dominated by truncated and multi-step
reasoning drafts where a late burst of uncertainty is plausible; on GSM8K
and MMLU almost every response has some high-entropy window (formatting
choices, the first word), so the peak saturates near its 2.32-bit maximum
for right and wrong drafts alike. Sustained uncertainty (the mean, or the
share of tokens above 1 bit) separates them better.

## 7. What this means for the design

1. **The savings are real but are not routing savings.** On every dataset
   `always_drafter` is the cheapest point and the router sits within one
   accuracy point of it. The honest one-line claim is "a cheap drafter
   handles 93 to 98% of this traffic acceptably", not "entropy picks the
   right requests to escalate".
2. **The windowed early-exit statistic is the wrong statistic.** It is the
   one that enables speculative execution (decide before the draft ends),
   so there is a real tension: the mean needs the whole draft, which costs
   latency on escalations. A middle ground worth testing is a windowed
   mean over the last N tokens checked at the end, or a running mean with a
   minimum token count before any decision.
3. **Threshold 2.0 is near the ceiling.** With top-5 logprobs the maximum
   per-token entropy is log2(5) = 2.32 bits. T=2.25 and T=2.50 are
   effectively unreachable, which is why those sweep rows are degenerate.
4. **The judge-based set cannot evaluate routers.** With 11 positives and a
   reference model that is correct by construction, no method can be
   distinguished from random. The answer-key sets are the ones to calibrate
   on.
5. **The published 91.6% was not reproducible.** Replaying the original
   trace file (`collected.pre_logprob.jsonl.gz`) with the current tooling
   gives 84.8% at T=2.0; the current trace gives 82.9%. No committed code
   or data produces 91.6%.

## 8. OpenAI logprob artifact and an open model

For 18.7% of project-set records (65.5% of creative prompts) OpenAI reports
`logprob: -9999` for at least one sampled token, meaning the sampled token
was outside the requested top-5. A min-logprob router cannot be made more
lenient than this sentinel, which caps `confidence_threshold` at 61.7% cost
reduction on the project set. The same event occurs on 28.7% of GSM8K
records and 74.3% of MMLU records (1.7% of MMLU tokens).

The sentinel is an API encoding, not a model property. The collector now
records `outside_top_k` per token (true when the sampled token is absent
from the returned alternatives) so the event can be measured on any
OpenAI-compatible server. Section 8.1 reports the same measurement on a
locally served open model.

### 8.1 Open model (qwen2.5-coder:7b via Ollama)

200 prompts (50 per category) collected drafter-only against a local Ollama
server with top-5 logprobs and a 512-token cap (33 responses hit the cap).
56,032 tokens. No heavyweight or judge, so there is no accuracy here; this
section is about the signal, not the router.

| Measure | gpt-4.1-nano (OpenAI) | qwen2.5-coder:7b (Ollama) |
|---|---|---|
| Sentinel `-9999` logprobs | present | none; minimum reported logprob -12.47 |
| Sampled token outside top-5, share of tokens | 0.49% creative, 0.01% code, 0.04% reasoning, 0.00% factual | 7.18% creative, 1.06% code, 0.70% reasoning, 1.01% factual |
| Records with at least one such token | 65.5% creative, 5.5% code, 12.8% reasoning, 0.0% factual | 100% creative, 84% code, 68% reasoning, 10% factual |
| Logprob of those tokens | unknown (masked by sentinel) | median -4.42, p10 -6.43, p90 -3.18 |
| Min-logprob router escalation at -5 / -8 / -10 / -15 | 47.0% / 22.4% / 19.3% / 18.7% (hard floor) | 61.5% / 15.5% / 2.0% / 0.0% |
| Entropy router escalation at T=2.0 | 7.7% | 24.5% |

Reading: the underlying event (the sampled token was not among the top five
alternatives) is common on any model that samples at temperature, far more
so on this one than the OpenAI drafter. What is specific to OpenAI is the
encoding: a sentinel that destroys the value. Because Ollama reports the real
logprob, the min-logprob baseline has no ceiling on the open model and can
be tuned all the way to 0% escalation. The confidence-threshold ceiling
reported in section 8 is therefore an API artifact, not a property of
logprob-based routing. Two of the 1,788 flagged tokens have logprob near 0;
they are byte-split Unicode fragments whose string form does not match the
alternatives list, a detection limitation rather than a sampling event.

The entropy router at T=2.0 would escalate 24.5% of this model's drafts
against 7.7% for the OpenAI drafter, and mean entropy on creative prompts is
1.07 bits against 0.57 overall. A threshold calibrated on one drafter does
not transfer to another; each drafter needs its own sweep.

Comparison outputs: `benchmarks/results/compare_qwen.csv` (accuracy columns
are meaningless for this unlabeled set; escalation rates and the artifact
table are the point).

## 9. Round 2: independent replication on new samples and a third benchmark

Round 1 measured everything once, on one seeded sample per benchmark. Round
2 drew a second, disjoint sample of each (seed 7, 500 prompts, source rows
from round 1 excluded via `fetchpublic -exclude`) and added ARC-Challenge
(500 of its 1,172 test questions, three to five choices, answer key). Same
models, same prompts formats, same grading. Collection cost USD 2.99 at list
prices for the three sets (token counts in the traces).

| Dataset | Prompts | Drafter wrong | Heavyweight wrong | Trace file |
|---|---|---|---|---|
| GSM8K, second sample | 500 | 50 (10.0%) | 21 (4.2%) | `collected_gsm8k_r2.jsonl.gz` |
| MMLU, second sample | 500 | 116 (23.2%) | 46 (9.2%) | `collected_mmlu_r2.jsonl.gz` |
| ARC-Challenge | 500 | 37 (7.4%) | 18 (3.6%) | `collected_arc_r2.jsonl.gz` |

Nine ARC prompts failed on a transient DNS timeout during collection and
were re-collected by rerunning the same command (the collector resumes).

### 9.1 Does the router result replicate?

Served accuracy at the production threshold against the same baselines,
round 1 next to round 2 (bootstrap 95% CI in parentheses):

| Dataset | Method | Escalation | Served accuracy | Cost reduction |
|---|---|---|---|---|
| GSM8K round 1 (n=300) | entropy router T=2.0 | 5.3% | 93.0% (90.0 to 95.7) | 84.3% |
| | random, same rate | 5.3% | 92.7% (89.3 to 95.3) | 84.6% |
| | mean-entropy router, same rate | 5.3% | 94.3% (91.7 to 96.7) | 83.9% |
| | always drafter / always heavyweight | 0% / 100% | 92.7% / 95.3% | 90.2% / 0% |
| GSM8K round 2 (n=500) | entropy router T=2.0 | 6.8% | 90.4% (87.6 to 92.8) | 81.9% (78.9 to 84.4) |
| | random, same rate | 6.4% | 90.8% (88.2 to 93.2) | 82.7% (80.1 to 85.0) |
| | mean-entropy router, same rate | 6.8% | 92.2% (89.6 to 94.4) | 80.7% (77.3 to 83.8) |
| | always drafter / always heavyweight | 0% / 100% | 90.0% / 95.8% | 89.4% / 0% |
| MMLU round 1 (n=300) | entropy router T=2.0 | 18.7% | 77.3% (72.7 to 82.0) | 75.2% |
| | random, same rate | 18.7% | 76.7% (72.0 to 81.3) | 74.8% |
| | mean-entropy router, same rate | 18.3% | 79.3% | 72.2% |
| | always drafter / always heavyweight | 0% / 100% | 75.7% / 89.7% | 92.4% / 0% |
| MMLU round 2 (n=500) | entropy router T=2.0 | 19.0% | 78.2% (74.8 to 81.8) | 73.7% (69.9 to 77.4) |
| | random, same rate | 19.2% | 79.2% (75.6 to 82.6) | 73.1% (69.1 to 76.9) |
| | mean-entropy router, same rate | 19.0% | 81.6% (78.4 to 85.0) | 68.1% (63.7 to 72.5) |
| | always drafter / always heavyweight | 0% / 100% | 76.8% / 90.8% | 92.8% / 0% |
| ARC-Challenge (n=500) | entropy router T=2.0 | 13.2% | 93.0% (90.6 to 95.2) | 79.0% (75.6 to 82.2) |
| | random, same rate | 12.8% | 93.2% (91.0 to 95.2) | 79.6% (76.4 to 82.6) |
| | mean-entropy router, same rate | 13.2% | 93.4% (91.2 to 95.6) | 78.1% (74.7 to 81.3) |
| | always drafter / always heavyweight | 0% / 100% | 92.6% / 96.4% | 92.6% / 0% |
| | oracle | 7.4% | 98.2% (97.0 to 99.2) | 84.4% (81.6 to 86.9) |

Round 1 MMLU mean-entropy row is quoted without an interval because it was
not bootstrapped in round 1's summary. All other figures come from
`compare_<dataset>.json`.

Replicated: on every new sample the windowed router's served accuracy is
0.2 to 1.0 points below the rate-matched random router and 0.4 to 1.4
points above always-drafter. The mean-entropy router is 1.4 and 2.4 points
above random on GSM8K and MMLU, with overlapping intervals, and 0.2 points
above it on ARC.

### 9.2 Does the signal-quality result replicate?

AUROC of each score as a detector of wrong drafts, round 1 and round 2 side
by side. Chance is 0.5.

| Score | GSM8K r1 (22 pos.) | GSM8K r2 (50 pos.) | MMLU r1 (73 pos.) | MMLU r2 (116 pos.) | ARC (37 pos.) |
|---|---|---|---|---|---|
| entropy_window_peak (production) | 0.509 (0.357 to 0.649) | 0.569 (0.483 to 0.658) | 0.474 (0.399 to 0.551) | 0.445 (0.388 to 0.503) | 0.519 (0.423 to 0.617) |
| mean_entropy | 0.760 (0.650 to 0.858) | 0.805 (0.734 to 0.867) | 0.626 (0.547 to 0.698) | 0.695 (0.647 to 0.743) | 0.718 (0.645 to 0.795) |
| frac_tokens_above_1bit | 0.720 (0.597 to 0.830) | 0.796 (0.721 to 0.863) | 0.624 (0.550 to 0.694) | 0.681 (0.631 to 0.730) | 0.712 (0.634 to 0.790) |
| neg_min_logprob | 0.621 (0.493 to 0.742) | 0.693 (0.608 to 0.778) | 0.682 (0.611 to 0.745) | 0.655 (0.600 to 0.710) | 0.604 (0.495 to 0.703) |

Replicated. The production statistic's interval covers 0.5 on all five
answer-key samples, and on MMLU round 2 (the largest sample, 116 positives)
it sits below chance at 0.445 with an interval that only just reaches 0.5:
if anything, high-peak drafts are slightly more likely to be right. Mean entropy is informative on all
five (lowest interval bound 0.547). The ordering mean > fraction above 1
bit > min logprob > window peak holds on every sample except round 1 MMLU,
where min logprob was best.

## 10. A learned router, trained on round 1 and tested on round 2

### 10.1 Setup

Features are computed in Go (`benchmarks/internal/sweep/features.go`) from
the drafter trace and exported with `benchmarks/cmd/features`; the same Go
code computes them at replay time, so training and inference cannot
disagree on a feature. The 22 features: mean, max and standard deviation
of per-token entropy; fraction of tokens above 0.5, 1.0 and 1.5 bits; the
production window peak; mean entropy of the last 10 tokens; token count and
its log; response length in characters; prompt tokens; minimum and mean
sampled-token logprob (sentinel tokens excluded, floor -20); fraction of
tokens sampled outside the top 5; and a one-hot prompt category.

`benchmarks/train_router.py` fits a standardised logistic regression and a
gradient-boosted tree ensemble (sklearn, 150 trees of depth 2) on the 600
round 1 GSM8K and MMLU records (95 positives), exports both to JSON, and
`benchmarks/internal/sweep/learned.go` evaluates them in Go. A parity test
checks the Go evaluator against the Python probabilities on 50 records to
1e-9. Calibration is isotonic regression on 5-fold out-of-fold
predictions; operating points are the largest acceptance sets whose served
error (wrong accepted drafts plus wrong heavyweight answers on escalations,
with a +1 finite-sample correction) stays under a target on those folds.
The round 2 sets are never seen during training or calibration.

### 10.2 Signal quality on held-out data

| Score | GSM8K r2 | ARC | MMLU r2 | Project set (judge, 11 pos.) |
|---|---|---|---|---|
| learned, logistic | 0.808 (0.736 to 0.873) | 0.748 (0.665 to 0.822) | 0.715 (0.662 to 0.767) | 0.741 (0.600 to 0.876) |
| learned, gradient boosting | 0.794 (0.722 to 0.860) | 0.719 (0.632 to 0.801) | 0.701 (0.647 to 0.752) | 0.669 (0.543 to 0.789) |
| mean_entropy alone | 0.805 (0.734 to 0.867) | 0.718 (0.645 to 0.795) | 0.695 (0.647 to 0.743) | 0.515 (0.382 to 0.643) |
| entropy_window_peak (production) | 0.569 (0.483 to 0.658) | 0.519 (0.423 to 0.617) | 0.445 (0.388 to 0.503) | 0.684 (0.590 to 0.779) |

The learned router beats the production statistic on every answer-key set
by 0.23 to 0.27 AUROC with non-overlapping intervals. It does not beat mean
entropy alone: the two differ by 0.003 to 0.030 and every interval overlaps.
With 600 training records the model has learned, in effect, "mean entropy
plus a little". The standardised logistic coefficients confirm it: token
count (+0.54), entropy standard deviation (+0.47), mean entropy (+0.37) and
fraction above 0.5 bits (+0.37) carry the weight; the window peak gets a
negative coefficient (-0.30) once the others are present.

Trained on all 2,100 answer-key records pooled (round 1 plus round 2, 298
positives) and scored out of fold, the logistic model reaches 0.794 (0.767
to 0.820) against 0.719 (0.688 to 0.748) for mean entropy on the same
records, intervals disjoint. More labels help; that model is reported for
context only, since it has no held-out test set
(`benchmarks/models/router_pooled_report.json`).

### 10.3 Cost and accuracy at a matched escalation rate

Thresholds set so each method escalates exactly what the windowed router
escalates on that dataset:

| Dataset | Method | Served accuracy | Cost reduction | USD per 1k |
|---|---|---|---|---|
| GSM8K r2 (esc. 6.8%) | learned, logistic | 92.4% (90.0 to 94.6) | 80.0% (76.5 to 83.3) | 0.374 |
| | mean-entropy router | 92.2% (89.6 to 94.4) | 80.7% (77.3 to 83.8) | 0.360 |
| | entropy router T=2.0 | 90.4% (87.6 to 92.8) | 81.9% (78.9 to 84.4) | 0.339 |
| | random | 90.8% (88.2 to 93.2) | 82.7% (80.1 to 85.0) | 0.325 |
| ARC (esc. 13.2%) | learned, logistic | 94.0% (91.8 to 96.0) | 78.0% (74.7 to 81.2) | 0.323 |
| | mean-entropy router | 93.4% (91.2 to 95.6) | 78.1% (74.7 to 81.3) | 0.322 |
| | entropy router T=2.0 | 93.0% (90.6 to 95.2) | 79.0% (75.6 to 82.2) | 0.309 |
| | random | 93.2% (91.0 to 95.2) | 79.6% (76.4 to 82.6) | 0.300 |
| MMLU r2 (esc. 19.0%) | learned, logistic | 82.2% (79.0 to 85.6) | 66.0% (61.0 to 70.9) | 0.739 |
| | mean-entropy router | 81.6% (78.4 to 85.0) | 68.1% (63.7 to 72.5) | 0.692 |
| | entropy router T=2.0 | 78.2% (74.8 to 81.8) | 73.7% (69.9 to 77.4) | 0.571 |
| | random | 79.2% (75.6 to 82.6) | 73.1% (69.1 to 76.9) | 0.584 |

The learned router is above random by 1.6 points on GSM8K, 0.8 on ARC and
3.0 on MMLU, always within overlapping intervals, and it costs more at the
same escalation rate, consistent with its positive token-count coefficient:
it prefers long, high-variance drafts, which are also the expensive ones
to replace.
The honest summary: a whole-trace router does beat random, by a small
margin that 500-prompt samples cannot pin down, and a learned one is no
better than the one-line mean-entropy rule at this training size.

### 10.4 Calibrated operating points do not transfer across datasets

The 8% served-error operating point chosen on the round 1 folds (tau 0.082
on the calibrated probability) gives, on held-out data:

| Dataset | Escalation | Served error | Target met | Cost reduction |
|---|---|---|---|---|
| GSM8K r2 | 19.0% | 6.4% | yes | 63.8% |
| ARC | 66.8% | 3.8% | yes, by escalating 66.8% of requests where the oracle needs 7.4% | 22.6% |
| MMLU r2 | 78.4% | 11.0% | no (heavyweight alone is 9.2% wrong) | 9.6% |

The 10% point (tau 0.312) behaves the same way: 7.4% served error on
GSM8K r2 at 7.2% escalation, 5.6% on ARC at 22.8%, and 14.6% on MMLU r2 at
40.4%. The guarantee held where the held-out distribution matched the
calibration mixture (GSM8K) and failed in both directions where it did
not. The calibration split was a 50/50 mix of GSM8K and MMLU; ARC is easier
than that mix and MMLU round 2 is harder than the 8% target allows at all.
Risk control of this kind needs a calibration set drawn from the traffic
it will govern, which is the usual exchangeability condition and a
practical reason to calibrate per route rather than globally.

### 10.5 Prefix-only routers are at chance

The same pipeline restricted to the first 20 or 50 drafter tokens (what an
early-exit router would see) gives held-out AUROC 0.488, 0.483 and 0.586
(20 tokens) and 0.591, 0.518 and 0.622 (50 tokens) on GSM8K r2, ARC and
MMLU r2. No feature combination available early in the draft predicts a
wrong answer on these tasks; the information arrives with the answer. This
is the measured form of the tension noted in section 7: the latency
features of the design (early exit, speculative execution) depend on a
prefix signal that does not exist here.

Plots: `tradeoff_gsm8k_r2.png`, `tradeoff_mmlu_r2.png`, `tradeoff_arc_r2.png`,
with bootstrap intervals as error bars on every bootstrapped point.

## 11. Speculative execution, measured by replay through the executor

`benchmarks/cmd/replayspec` feeds each recorded trace through the
production `speculative.Executor` with a fake heavyweight client. Every
token becomes a stream chunk whose five synthetic alternatives reproduce
the recorded entropy exactly under `entropy.ComputeEntropy`, so the
executor sees what the gateway saw. The escalation count it produces equals
the offline replay's on every trace, which is the consistency check.

At the production soft multiplier (0.8, so a speculative heavyweight call
starts at the first token above 1.6 bits):

| Trace | Requests | Escalations | Speculative triggers | Cancelled | Used on escalation | Escalated with no head start | Head start p50 / p90 | Cancelled calls per 1k requests |
|---|---|---|---|---|---|---|---|---|
| Project set | 519 | 40 | 366 (70.5%) | 353 (96.4%) | 13 | 27 | 36 ms / 162 ms | 680 |
| GSM8K r1 | 300 | 16 | 263 (87.7%) | 260 (98.9%) | 3 | 13 | 26 ms / 70 ms | 867 |
| MMLU r1 | 300 | 56 | 258 (86.0%) | 241 (93.4%) | 17 | 39 | 27 ms / 88 ms | 803 |
| GSM8K r2 | 500 | 34 | 444 (88.8%) | 438 (98.6%) | 6 | 28 | 37 ms / 44 ms | 876 |
| ARC | 500 | 66 | 444 (88.8%) | 428 (96.4%) | 16 | 50 | 30 ms / 70 ms | 856 |
| MMLU r2 | 500 | 95 | 432 (86.4%) | 400 (92.6%) | 32 | 63 | 29 ms / 59 ms | 800 |

Three things the counters would have shown had they been read:

1. **The trigger is per token, not per window.** The executor starts the
   heavyweight when any single token exceeds the soft threshold
   (`executor.go`, `te.Entropy > softThreshold`), and almost every draft has
   one such token (a formatting choice, a first word). Trigger rates are 70
   to 89% of requests against escalation rates of 5 to 19%.
2. **The head start is tens of milliseconds.** Head start is the drafter
   time between the trigger token and the escalation token, estimated from
   the record's drafter latency per token. Its median is 26 to 37 ms. And
   because the executor checks the hard threshold before the soft one on
   each token, a token that jumps straight past T escalates with no
   speculative call at all: 27 of the project set's 40 escalations, 63 of
   MMLU r2's 95.
3. **Waste is bounded above by the entire saving.** If each cancelled call
   were billed to completion, the cancelled calls alone would cost 80 to 88%
   of the always-heavyweight bill, which is more than the router saves.
   What a cancelled stream actually bills depends on how many tokens the
   provider generated before the cancel arrived, which an offline replay
   cannot measure; the figure is an upper bound, and the lower bound is not
   zero.

Lowering the multiplier does not help: on the project set, 0.5 triggers on
77.5% of requests and cancels 94.8%; 0.9 still triggers on 63.4% and
cancels 97.6% (`speculative_project.json`). The Phase 4 exit criterion
("wasted compute from cancelled speculative calls under 10% of escalation
cost") is not met by any multiplier on any trace. The recommendation in
`docs/STREAMING_DESIGN.md` is to disable speculative execution until a
prefix signal exists.

## 12. Semantic cache, measured by replay through the store

`benchmarks/cmd/replaycache` drives the production `cache.VectorStore`
(same key text, same OpenAI embedding client, same 0.95 cosine threshold)
with an in-memory index and key-value store in place of Qdrant and Redis,
replaying each trace's prompts in order and inserting the drafts the T=2.0
router would have accepted, which is the gateway's insertion rule.

| Trace | Prompts | Inserted | Hits at 0.95 | Hits serving a wrong answer | Nearest-neighbour similarity p50 / p99 / max | Lookup p50 / p99 |
|---|---|---|---|---|---|---|
| Project set | 519 | 479 | 0 (0.0%) | 0 | 0.535 / 0.799 / 0.878 | 197 ms / 374 ms |
| GSM8K r1 | 300 | 284 | 0 (0.0%) | 0 | 0.600 / 0.731 / 0.763 | 199 ms / 406 ms |
| MMLU r1 | 300 | 241 | 3 (1.0%) | 1 | 0.583 / 0.946 / 1.000 | 201 ms / 360 ms |

Findings:

- **Hit rate on unique benchmark traffic is 0 to 1%.** None of the project
  set's prompts comes within 0.878 of another. The three MMLU hits are two
  exact duplicates (MMLU contains repeated items; cosine 1.000) and one
  near-duplicate at 0.959 from `moral_scenarios`, where every question shares
  a long template and only the two scenarios differ. That hit served the
  cached answer B to a question whose answer is A.
- **Lowering the threshold would serve wrong answers.** At 0.90 the MMLU
  replay would hit 30 prompts and serve a wrong cached answer on 19 of them;
  at 0.80, 33 and 20. The templated-prompt failure mode is real and the
  0.95 threshold is the right side of it on this data, with a margin of one
  bad hit in 300.
- **Lookup costs about 200 ms, not 10 to 50 ms.** The embedding API call
  dominates (p50 197 to 201 ms, p99 360 to 406 ms from this machine); the
  brute-force search over a few hundred vectors is negligible and Qdrant
  would not change the embedding latency. With the cache enabled every
  request pays this before the drafter starts. The Phase 5 exit criterion
  ("cached responses under 50 ms") is not met for the lookup path, and the
  system design's "10 to 15 ms" miss overhead was never measured.
- A second pass over the same project-set traffic hits on all 479 inserted
  prompts, confirming the store, index and key-value plumbing round-trip
  (`cache_replay_project.json`, `repeat_pass_hits`).

Embedding spend for the three replays was about USD 0.03.

## 13. Open model as drafter with exact-match grading (GSM8K)

Round 1 ran the open model drafter-only. Round 2 ran qwen2.5-coder:7b
(Q4_K_M via Ollama on an RTX 4050 laptop GPU, 6 GB) as the drafter with
gpt-4.1 as the heavyweight on the round 1 GSM8K sample, graded both
against the answer key, and ran every analysis on it
(`collected_gsm8k_qwen2.5-coder-7b.jsonl.gz`, `compare_gsm8k_qwen.*`).

| Method | Escalation | Served accuracy | Notes |
|---|---|---|---|
| always drafter (qwen2.5-coder:7b) | 0% | 83.7% (79.3 to 87.7) | 49 of 300 wrong |
| always heavyweight (gpt-4.1) | 100% | 97.0% (95.0 to 98.7) | 9 wrong |
| entropy router T=2.0 | 28.0% | 87.3% (83.3 to 91.0) | TP 13, FP 71, FN 36 |
| random, same rate | 28.7% | 88.0% (84.3 to 91.7) | |
| mean-entropy router, same rate | 28.0% | 90.3% (86.7 to 93.7) | TP 25 |
| oracle | 16.3% | 98.0% (96.3 to 99.3) | |

| Score | AUROC |
|---|---|
| entropy_window_peak | 0.516 (0.426 to 0.606) |
| mean_entropy | 0.727 (0.649 to 0.798) |
| frac_tokens_above_1bit | 0.731 (0.650 to 0.805) |
| neg_min_logprob | 0.592 (0.504 to 0.674) |

The pattern transfers to a different model family: the windowed statistic
is at chance, the trace mean is informative, the router at T=2.0 does not
beat random. Two things are specific to the open model:

- **The min-logprob baseline has no ceiling.** Ollama reports real
  logprobs for tokens sampled outside the top 5 (340 of 77,496 tokens, in
  52% of records), so the confidence-threshold sweep runs the full range:
  100% escalation at -1, 49.3% at -5, 8.3% at -8, 1.7% at -10, 0.0% at -15.
  This completes the round 1 argument that the OpenAI ceiling is an API
  encoding artifact. With real values the score is still weak (AUROC 0.59).
- **T=2.0 does not transfer.** It escalates 28.0% of this drafter's
  responses against 5.3% for gpt-4.1-nano on the same 300 prompts, at lower
  accuracy than random at the same rate.

Caveats: the cost columns price the local drafter at gpt-4.1-nano list
rates as a stand-in, so cost reduction is comparable in shape but not a
measurement of this deployment; the drafter's p50 latency of 23.1 s per
draft reflects the laptop GPU; and the speculative replay shows the same
behaviour as the hosted drafter (74.7% trigger, 95.5% cancel) with a larger
head start (p50 545 ms) only because the drafter is slow.

The whole-trace learned router was not retrained for this drafter; its
features assume the gpt-4.1-nano feature distribution.

## 14. Reproducing

```bash
# Replay sweeps and bootstrap CIs from the committed traces. No API calls.
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz       -output benchmarks/results/compare.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k.jsonl.gz -output benchmarks/results/compare_gsm8k.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_mmlu.jsonl.gz  -output benchmarks/results/compare_mmlu.csv

# Round 2 replication sets and the learned routers (models are committed; -models adds their rows and AUROC)
M=benchmarks/models/router_full_logistic.json,benchmarks/models/router_full_gbt.json,benchmarks/models/router_prefix20_logistic.json,benchmarks/models/router_prefix50_logistic.json
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k_r2.jsonl.gz -output benchmarks/results/compare_gsm8k_r2.csv -models $M
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_mmlu_r2.jsonl.gz  -output benchmarks/results/compare_mmlu_r2.csv  -models $M
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_arc_r2.jsonl.gz   -output benchmarks/results/compare_arc_r2.csv   -models $M
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz          -output benchmarks/results/compare.csv          -models $M
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k_qwen2.5-coder-7b.jsonl.gz -output benchmarks/results/compare_gsm8k_qwen.csv
python benchmarks/plot_tradeoff.py benchmarks/results/compare_gsm8k_r2.csv benchmarks/results/tradeoff_gsm8k_r2.png "GSM8K replication (n=500, held out)"

# Each run also writes <output>_by_category.csv and <output>.json (headline numbers with CIs).
# -output-only-pricing reproduces the original cost model that ignored prompt tokens.

# Retrain the learned routers (Python, offline; the Go parity test then checks the export)
go run ./benchmarks/cmd/features -input benchmarks/results/collected_gsm8k.jsonl.gz,benchmarks/results/collected_mmlu.jsonl.gz -output benchmarks/results/features_train.csv
python benchmarks/train_router.py --train benchmarks/results/features_train.csv --name full
go run ./benchmarks/cmd/features -input benchmarks/results/collected_gsm8k.jsonl.gz,benchmarks/results/collected_mmlu.jsonl.gz -output benchmarks/results/features_train_prefix20.csv -prefix-tokens 20
python benchmarks/train_router.py --train benchmarks/results/features_train_prefix20.csv --name prefix20 --prefix-tokens 20
go test ./benchmarks/internal/sweep/ -run Learned

# Speculative execution replay through the production executor (no API calls)
go run ./benchmarks/cmd/replayspec -input benchmarks/results/collected.jsonl.gz
go run ./benchmarks/cmd/replayspec -input benchmarks/results/collected_mmlu_r2.jsonl.gz

# Semantic cache replay (embedding calls, about one cent per thousand prompts)
OPENAI_API_KEY=... go run ./benchmarks/cmd/replaycache -input benchmarks/results/collected.jsonl.gz -repeat
OPENAI_API_KEY=... go run ./benchmarks/cmd/replaycache -input benchmarks/results/collected_mmlu.jsonl.gz

# Re-collect (real API spend; non-deterministic sampling means numbers will move within the CIs)
go run ./benchmarks/cmd/fetchpublic -dataset gsm8k -n 300 -seed 42
go run ./benchmarks/cmd/fetchpublic -dataset mmlu  -n 300 -seed 42
go run ./benchmarks/cmd/fetchpublic -dataset gsm8k -n 500 -seed 7 -id-prefix gsm8k_r2_ -exclude benchmarks/testdata/gsm8k_300.json -output benchmarks/testdata/gsm8k_r2_500.json
go run ./benchmarks/cmd/fetchpublic -dataset mmlu  -n 500 -seed 7 -id-prefix mmlu_r2_  -exclude benchmarks/testdata/mmlu_300.json  -output benchmarks/testdata/mmlu_r2_500.json
go run ./benchmarks/cmd/fetchpublic -dataset arc   -n 500 -seed 7 -id-prefix arc_r2_   -output benchmarks/testdata/arc_r2_500.json
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/gsm8k_300.json    -output benchmarks/results/collected_gsm8k.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/mmlu_300.json     -output benchmarks/results/collected_mmlu.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/gsm8k_r2_500.json -output benchmarks/results/collected_gsm8k_r2.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/mmlu_r2_500.json  -output benchmarks/results/collected_mmlu_r2.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/arc_r2_500.json   -output benchmarks/results/collected_arc_r2.jsonl

# Open model as drafter (Ollama serving qwen2.5-coder:7b locally; heavyweight and grading as above)
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/gsm8k_300.json \
  -drafter-base-url http://localhost:11434/v1 -drafter-model qwen2.5-coder:7b -drafter-key-env NONE \
  -heavy-model gpt-4.1 -concurrency 2 -output benchmarks/results/collected_gsm8k_qwen2.5-coder-7b.jsonl

# Drafter-only traces from a local OpenAI-compatible server (no API spend)
go run ./benchmarks/cmd/collect -drafter-only -drafter-base-url http://localhost:11434/v1 \
  -drafter-model qwen2.5-coder:7b -drafter-key-env NONE -limit 50 -max-tokens 512 \
  -output benchmarks/results/collected_qwen2.5-coder-7b.jsonl
```
