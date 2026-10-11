# Draft-Thinker

A cost-aware LLM gateway in Go. A cheap drafter model answers first; the gateway watches the drafter's token-level confidence (Shannon entropy over returned logprobs) and escalates to an expensive model when confidence drops. Every number below is regenerated from committed data by a command in this file.

## The Problem

LLM-powered applications send 100% of traffic to frontier models regardless of query complexity. "What are your hours?" costs the same as "Explain the tradeoffs between B-tree and LSM-tree storage engines." This is wasteful in three ways:

- **Cost**: most queries are answerable by models costing 10-50x less.
- **Rate limits**: at high throughput, frontier-model quotas become the bottleneck, not your application.
- **Blast radius**: a single provider outage takes down every request.

The hard part isn't routing, it's knowing *when* the cheap model is good enough without already having the right answer. Prompt classifiers ("is this question easy?") fail on distribution shift. A syntactically simple question can require complex reasoning depending on context.

## The Approach

Draft-Thinker analyzes the drafter model's own confidence signals during generation. Every token a model produces comes with log-probabilities for its top candidates. High entropy across those candidates means the model is guessing. Low entropy means it's confident.

### Entropy-Based Routing

The core mechanism computes Shannon entropy over the drafter's top-5 token logprobs using a 10-token sliding window. A calibrated threshold T determines the routing decision: pass the draft or escalate. The threshold is set by sweeping a benchmark dataset and inspecting the accuracy-cost curve.

The known failure mode is confident hallucination: the drafter produces a wrong answer with low entropy. This is not hypothetical. On the project benchmark all 11 unacceptable drafts have mean entropy below 0.5 bits and the router catches one of them. See [Results](docs/RESULTS.md) for what the evaluation found about the signal.

### Speculative Execution

Naive draft-then-verify is serial. Hard questions pay double latency. Draft-Thinker fires the heavyweight model in parallel when early tokens show elevated (but not yet critical) uncertainty. If the drafter recovers, the heavyweight call is canceled. If not, the heavyweight already has a head start.

### Semantic Cache

Previously verified prompt-response pairs are cached via embedding similarity (OpenAI text-embedding-3-small + Qdrant). If an incoming prompt is semantically similar (cosine > 0.95) to a cached entry, the response is returned directly. Only draft-accepted responses are cached.

### Graceful Degradation

Provider calls fail. When the drafter errors or times out, the request goes straight to the heavyweight. When the heavyweight fails after an escalation decision, the drafter is re-run and its answer is served uncached and flagged. Both fallbacks are on by default (`fallback:` in `config.yaml`), surface on the `X-Routing-Decision` and `X-Fallback-Reason` headers, and are counted in Prometheus so an upstream incident shows up as a shift in routing mix rather than a wall of 5xx.

## Tech Stack

- **Gateway** (Go `net/http`): goroutines for concurrent I/O. The bottleneck is API latency, not compute.
- **Entropy engine** (Go `math`): pure math, no reason to cross a language boundary.
- **Drafter** (OpenAI gpt-4.1-nano): fast, cheap, returns logprobs.
- **Heavyweight** (OpenAI gpt-4.1): escalation target. Real API costs for honest benchmarking.
- **Vector cache** (Qdrant) and **KV store** (Redis): semantic cache and TTLs.
- **Observability** (Prometheus + Grafana): cost per request, entropy distributions, routing and fallback decisions, cache hit rate.
- **Deployment** (Docker Compose): single command spins up gateway, Redis, Qdrant, Grafana.

No Python in the hot path. The draft-verify state machine is a Go switch statement.

## Results

Full write-up with methodology, baselines and confidence intervals: [docs/RESULTS.md](docs/RESULTS.md). Headline numbers, all replayed from committed traces (gpt-4.1-nano drafter, gpt-4.1 heavyweight, input and output tokens priced at list rates, bootstrap 95% CIs from 2000 resamples):

| Dataset | Labels | Method | Escalation | Served accuracy | Cost reduction vs all-heavyweight | USD per 1k requests |
|---|---|---|---|---|---|---|
| Project set, 519 prompts | LLM-as-judge | entropy router, T=2.0 | 7.7% | 98.1% (96.7 to 99.0) | 82.9% (79.6 to 86.0) | 0.534 |
| | | random, same rate | 7.9% | 98.1% (96.7 to 99.0) | 83.9% (80.6 to 86.7) | 0.504 |
| | | always drafter | 0% | 97.9% (96.5 to 99.0) | 92.6% (92.3 to 92.8) | 0.233 |
| GSM8K, 300 prompts | answer key | entropy router, T=2.0 | 5.3% | 93.0% (90.0 to 95.7) | 84.3% (81.4 to 87.0) | 0.293 |
| | | mean-entropy router, same rate | 5.3% | 94.3% (91.7 to 96.7) | 83.9% (80.1 to 86.9) | 0.301 |
| | | always drafter | 0% | 92.7% (89.3 to 95.3) | 90.2% (89.7 to 90.6) | 0.183 |
| | | always heavyweight | 100% | 95.3% (93.0 to 97.7) | 0% | 1.866 |
| MMLU, 300 prompts | answer key | entropy router, T=2.0 | 18.7% | 77.3% (72.7 to 82.0) | 75.2% (70.4 to 79.9) | 0.549 |
| | | random, same rate | 18.7% | 76.7% (72.0 to 81.3) | 74.8% (69.7 to 79.1) | 0.558 |
| | | always heavyweight | 100% | 89.7% (86.0 to 93.0) | 0% | 2.215 |

What this says, honestly: the cost savings are large, reproducible and come with intervals, but they come from the drafter being cheap and usually right. At the production threshold the windowed entropy statistic does not select wrong drafts better than random escalation on any of the three datasets (AUROC 0.68, 0.51, 0.47). Trace-level statistics that need the whole draft carry signal (mean entropy AUROC 0.76 on GSM8K); that is the next design iteration, not the shipped one.

### Gateway performance under load

Mock OpenAI upstream (20-token SSE responses, 5 ms per chunk), gateway on the same machine, speculative execution on, cache off. Regenerate with the commands under Load testing below.

| Scenario | Requests | Success | p50 | p95 | p99 | Routing decisions |
|---|---|---|---|---|---|---|
| confident (all drafts accepted) | 1500 at 50 req/s | 100% | 105.9 ms | 106.9 ms | 107.4 ms | accept 1500 |
| mixed (20% of drafts uncertain) | 1500 at 50 req/s | 100% | 106.3 ms | 108.4 ms | 109.8 ms | accept 1188, escalate 312 |
| flaky (20% drafter 503s, 5% drafter stalls, 2 s drafter timeout) | 1500 at 50 req/s | 100%, zero 5xx | 107.1 ms | 111.0 ms | 2107.9 ms | accept 895, escalate 249, fallback_heavyweight 356 |

The mock responds in about 100 ms (20 chunks at 5 ms), so p50 is upstream time and gateway overhead is in the low milliseconds. In the flaky run every injected drafter failure was served by the heavyweight; the p99 is the 2 s drafter timeout plus one heavyweight call, which is the cost of a stall rather than an error. The heavyweight is never faulted in this scenario, so `fallback_draft` does not appear; its path is covered by unit tests. Before this pass the mixed scenario's uncertain tokens carried 1.58 bits of entropy, below T=2.0, so the earlier load tests never exercised escalation.

### Earlier published figures

Earlier versions of this README reported 91.6% cost reduction, 94% acceptance and 98.2% accuracy on 518 prompts. Those figures could not be regenerated from any committed trace or code (the preserved original trace replays to 84.8% at T=2.0 with the current tooling) and have been retired. Details in [docs/RESULTS.md](docs/RESULTS.md) section 7.

## Quick Start

```bash
# Prerequisites: Go 1.25+, Docker, OpenAI API key

git clone https://github.com/trnahnh/draft-thinker.git
cd draft-thinker
go build -o draft-thinker ./cmd/gateway

docker compose up -d  # Redis, Qdrant, Grafana, Prometheus

export OPENAI_API_KEY=...
./draft-thinker --config config.yaml

curl -i http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "auto", "messages": [{"role": "user", "content": "What is 2+2?"}]}'
# X-Routing-Decision: accept | escalate | cache_hit | fallback_heavyweight | fallback_draft
```

## Reproducing the numbers

Replaying the committed traces makes no API calls and takes seconds:

```bash
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz       -output benchmarks/results/compare.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k.jsonl.gz -output benchmarks/results/compare_gsm8k.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_mmlu.jsonl.gz  -output benchmarks/results/compare_mmlu.csv
python benchmarks/plot_tradeoff.py   # and with the gsm8k / mmlu CSVs, see script header
```

Re-collecting traces hits the real API (roughly USD 3 for the project set, under USD 2 for GSM8K plus MMLU) and is non-deterministic; expect results to move within the published intervals:

```bash
go run ./benchmarks/cmd/fetchpublic -dataset gsm8k -n 300 -seed 42
go run ./benchmarks/cmd/fetchpublic -dataset mmlu  -n 300 -seed 42
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/prompts.json    -output benchmarks/results/collected.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/gsm8k_300.json -output benchmarks/results/collected_gsm8k.jsonl
OPENAI_API_KEY=... go run ./benchmarks/cmd/collect -prompts benchmarks/testdata/mmlu_300.json  -output benchmarks/results/collected_mmlu.jsonl
```

### Load testing

No API key needed. Terminal 1 runs the gateway against the mock upstream the load tester starts on port 9999:

```bash
go build -o draft-thinker ./cmd/gateway
OPENAI_API_KEY=mock ./draft-thinker --config config.loadtest.yaml
```

Terminal 2:

```bash
go run ./benchmarks/cmd/loadtest -scenario confident -rate 50 -duration 30s
go run ./benchmarks/cmd/loadtest -scenario mixed     -rate 50 -duration 30s
go run ./benchmarks/cmd/loadtest -scenario flaky     -rate 50 -duration 30s   # 20% drafter 503s, 5% drafter stalls
```

## Tests

```bash
go test ./...
go test -race ./...
go test -tags=integration ./...   # needs docker compose up
```

## Observability

`docker compose up -d` auto-provisions Grafana with a pre-built dashboard at `http://localhost:3000` (admin/admin) covering request rates, routing decisions (including fallbacks), latency percentiles, entropy distribution, cache hit rate, and speculative execution metrics. Prometheus scrapes the gateway at `/metrics` every 15 seconds. Metric reference: [docs/METRICS.md](docs/METRICS.md).

## Demo

Research showcase site presenting the calibration results and architecture: [draft-thinker.vercel.app](https://draft-thinker.vercel.app). Run locally with `cd web && npm install && npm run dev`.

## Documentation

- [Results](docs/RESULTS.md): datasets, baselines, confidence intervals, negative findings, reproduction commands
- [System Design](docs/SYSTEM_DESIGN.md): architecture, entropy algorithm, speculative execution, cache, failure handling
- [Development Phases](docs/PHASES.md): deliverables and exit criteria per phase
- [Metrics](docs/METRICS.md): Prometheus instruments and offline evaluation metrics
- [Enhancement Report](ENHANCEMENT_REPORT.md): what changed in the evaluation-hardening pass, before and after

## Inspired By

[Draft-Thinking: Learning Efficient Reasoning in Long Chain-of-Thought LLMs](https://arxiv.org/abs/2603.00578).

## Contact

**Anh Tran** -[anhdtran.forwork@gmail.com](mailto:anhdtran.forwork@gmail.com)

## License

MIT
