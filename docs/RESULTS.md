# Results: Entropy Router vs. Baselines

Every number in this document is produced by a command listed in section 9
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

---

## 1. Datasets

Three datasets, all committed so the sweeps replay offline:

| Dataset | Prompts | Labels | Drafter wrong | Heavyweight wrong | Trace file |
|---|---|---|---|---|---|
| Project set (4 categories) | 519 valid of 520 | LLM-as-judge (gpt-4.1) vs. heavyweight reference | 11 (2.1%) | n/a (reference answer) | `collected.jsonl.gz` |
| GSM8K test, seeded sample | 300 | exact numeric match to answer key | 22 (7.3%) | 14 (4.7%) | `collected_gsm8k.jsonl.gz` |
| MMLU test, seeded sample | 300 | exact A to D match to answer key | 73 (24.3%) | 31 (10.3%) | `collected_mmlu.jsonl.gz` |

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

## 9. Reproducing

```bash
# Replay sweeps and bootstrap CIs from the committed traces. No API calls.
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz       -output benchmarks/results/compare.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k.jsonl.gz -output benchmarks/results/compare_gsm8k.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_mmlu.jsonl.gz  -output benchmarks/results/compare_mmlu.csv

# Each run also writes <output>_by_category.csv and <output>.json (headline numbers with CIs).
# -output-only-pricing reproduces the original cost model that ignored prompt tokens.

# Re-collect (real API spend; non-deterministic sampling means numbers will move within the CIs)
go run ./benchmarks/cmd/fetchpublic -dataset gsm8k -n 300 -seed 42
go run ./benchmarks/cmd/fetchpublic -dataset mmlu  -n 300 -seed 42
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/gsm8k_300.json -output benchmarks/results/collected_gsm8k.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/mmlu_300.json  -output benchmarks/results/collected_mmlu.jsonl

# Drafter-only traces from a local OpenAI-compatible server (no API spend)
go run ./benchmarks/cmd/collect -drafter-only -drafter-base-url http://localhost:11434/v1 \
  -drafter-model qwen2.5-coder:7b -drafter-key-env NONE -limit 50 -max-tokens 512 \
  -output benchmarks/results/collected_qwen2.5-coder-7b.jsonl
```
