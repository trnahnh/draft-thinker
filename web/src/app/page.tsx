import Metric from "@/components/Metric";
import SweepTable from "@/components/SweepTable";
import CostBar from "@/components/CostBar";
import ConfusionMatrix from "@/components/ConfusionMatrix";
import Architecture from "@/components/Architecture";
import EntropyViz from "@/components/EntropyViz";
import StateMachine from "@/components/StateMachine";
import BenchmarkTable from "@/components/BenchmarkTable";
import AurocChart from "@/components/AurocChart";
import FactList from "@/components/FactList";
import {
  SpeculativeTable,
  CacheTable,
  LoadTestTable,
} from "@/components/ReplayTables";

function Panel({
  label,
  children,
  className = "",
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`border border-(--color-border) bg-(--color-surface) ${className}`}
    >
      <div className="border-b border-(--color-border) px-4 py-2.5 sm:px-5">
        <span className="font-mono text-[10px] uppercase tracking-[0.15em] text-(--color-text-dark)">
          {label}
        </span>
      </div>
      <div className="p-4 sm:p-5">{children}</div>
    </div>
  );
}

function Section({ index, title, sub }: { index: string; title: string; sub: string }) {
  return (
    <div className="flex items-baseline gap-3 pt-6 pb-1 first:pt-0">
      <span className="font-mono text-[10px] text-(--color-orange)">{index}</span>
      <span className="font-mono text-xs uppercase tracking-[0.15em] text-(--color-text)">
        {title}
      </span>
      <span className="hidden text-[11px] text-(--color-text-dark) sm:inline">
        {sub}
      </span>
      <span className="ml-2 h-px flex-1 bg-(--color-border)" />
    </div>
  );
}

const tradeoffs = [
  {
    title: "ROUTER VS RANDOM",
    color: "var(--color-red)",
    text: "At T=2.0 the entropy router is 0.2 to 1.0 accuracy points below a random router escalating the same share on all three held-out sets, and level with it on the project set. The savings come from the cheap drafter being usually right, not from the entropy signal choosing what to escalate.",
  },
  {
    title: "CONFIDENT HALLUCINATION",
    color: "var(--color-orange)",
    text: "Drafter produces wrong answers with low entropy. 10 of 11 bad drafts pass at T=2.0 on the project set; all 11 have mean entropy under 0.5 bits. On MMLU r2 the window peak sits below chance at 0.445.",
  },
  {
    title: "NO PREFIX SIGNAL",
    color: "var(--color-amber)",
    text: "Routers restricted to the first 20 or 50 drafter tokens reach held-out AUROC 0.48 to 0.62. The information that predicts a wrong draft arrives with the answer, which removes the basis for early exit and speculative execution on these tasks.",
  },
  {
    title: "SPECULATIVE WASTE",
    color: "var(--color-amber)",
    text: "Replayed through the executor: fires on 70 to 89% of requests, cancels 93 to 99% of those calls, median head start 26 to 37 ms. The trigger is per token, not per window, which the design doc had wrong. The under-10%-waste target is not met at any multiplier.",
  },
  {
    title: "CACHE CONSERVATISM",
    color: "var(--color-cyan)",
    text: "Cosine > 0.95, only draft-accepted responses cached. Replayed: 0 to 1% hits on unique benchmark prompts, 100% on repeats, lookup about 200 ms. At 0.90 MMLU would serve 19 wrong cached answers in 30 hits.",
  },
  {
    title: "CALIBRATION DOES NOT TRAVEL",
    color: "var(--color-green)",
    text: "An 8% served-error operating point calibrated on round 1 folds held on GSM8K r2 (6.4%), held wastefully on ARC by escalating 66.8%, and missed on MMLU r2 (11.0%, where the heavyweight alone is 9.2% wrong). Risk control needs calibration data from the traffic it governs.",
  },
];

export default function Home() {
  return (
    <div className="grid-overlay min-h-screen">
      <nav className="sticky top-0 z-50 border-b border-(--color-border) bg-bg/90 backdrop-blur-sm">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6">
          <div className="flex items-center gap-3">
            <span className="font-mono text-sm font-medium tracking-tight text-(--color-orange)">
              DRAFT-THINKER
            </span>
            <span className="hidden font-mono text-[10px] text-(--color-text-dark) sm:inline">
              COST-AWARE LLM GATEWAY
            </span>
          </div>
          <div className="flex items-center gap-3 font-mono text-[10px] text-(--color-text-dark) sm:gap-5">
            <span className="hidden items-center gap-1.5 sm:flex">
              <span className="h-1.5 w-1.5 bg-(--color-green)" />
              7 TRACES, 2919 RECORDS
            </span>
            <span className="hidden sm:inline">T=2.0</span>
            <a
              href="https://github.com/trnahnh/draft-thinker"
              target="_blank"
              rel="noopener noreferrer"
              className="text-(--color-cyan) transition-colors hover:text-(--color-text)"
            >
              SOURCE
            </a>
          </div>
        </div>
      </nav>

      <main className="mx-auto max-w-7xl space-y-4 px-4 py-6 sm:px-6 sm:py-8">
        <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
          <Metric
            value="82.9%"
            label="Cost Reduction"
            sublabel="95% CI 79.6 to 86.0, vs all-heavyweight, 519 prompts"
            accent="orange"
          />
          <Metric
            value="92.3%"
            label="Draft Acceptance"
            sublabel="479 of 519 prompts at T=2.0"
            accent="green"
          />
          <Metric
            value="98.1%"
            label="Served Accuracy"
            sublabel="LLM-as-judge, 95% CI 96.7 to 99.0"
            accent="cyan"
          />
          <Metric
            value="107ms"
            label="P99 Latency"
            sublabel="accept path, 50 req/s, mock upstream"
          />
        </div>

        <p className="max-w-4xl text-xs leading-relaxed text-(--color-text-dim)">
          The headline above is real and replays from a committed trace. The
          rest of this page is the evaluation that followed: random, oracle,
          always-drafter and learned-router baselines, three public
          benchmarks with answer keys, a disjoint replication of every
          result, and the speculative executor and semantic cache measured
          by replaying traces through their production code. Most of it is
          negative for the entropy signal, and all of it reproduces with one
          command from the repository.
        </p>

        <Section
          index="01"
          title="Verdict"
          sub="what the held-out evaluation established"
        />
        <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
          <Metric
            value="0.45 to 0.57"
            size="sm"
            label="Window-peak AUROC"
            sublabel="the statistic the gateway thresholds, 5 answer-key samples, chance is 0.5"
            accent="red"
          />
          <Metric
            value="0.70 to 0.81"
            size="sm"
            label="Mean-entropy AUROC"
            sublabel="same 5 samples, needs the whole draft"
            accent="green"
          />
          <Metric
            value="-0.2 to -1.0"
            size="sm"
            label="Router vs random, pts"
            sublabel="served accuracy at T=2.0, 3 held-out sets, same escalation rate"
            accent="red"
          />
          <Metric
            value="+0.8 to +3.0"
            size="sm"
            label="Learned vs random, pts"
            sublabel="logistic on 22 trace features, trained r1, tested r2, CIs overlap"
            accent="cyan"
          />
        </div>

        <Section
          index="02"
          title="Held-Out Evaluation"
          sub="GSM8K, MMLU, ARC-Challenge, exact-match graded, bootstrap CIs"
        />
        <div className="grid gap-4 lg:grid-cols-3">
          <Panel
            label="Served Accuracy at Matched Escalation (7 traces)"
            className="lg:col-span-2"
          >
            <BenchmarkTable />
          </Panel>
          <Panel label="Learned Router">
            <FactList
              facts={[
                { k: "Features", v: "22 from the drafter trace" },
                { k: "Models", v: "logistic, GBT (150 x depth 2)" },
                { k: "Train", v: "r1 GSM8K + MMLU, 600 records" },
                { k: "Test", v: "r2 GSM8K, MMLU, ARC, 1500" },
                { k: "Go / Python parity", v: "50 records to 1e-9" },
                {
                  k: "vs production AUROC",
                  v: "+0.23 to +0.27, CIs disjoint",
                  color: "var(--color-green)",
                },
                {
                  k: "vs mean entropy",
                  v: "+0.003 to +0.030, CIs overlap",
                  color: "var(--color-amber)",
                },
                {
                  k: "Prefix-only (20 / 50 tok)",
                  v: "AUROC 0.48 to 0.62",
                  color: "var(--color-red)",
                },
                { k: "Pooled 2100, OOF", v: "0.794 vs 0.719 mean" },
              ]}
              note="Features computed in Go and replayed in Go, so training and inference cannot disagree. Top standardised coefficients: token count +0.54, entropy std +0.47, mean entropy +0.37; the window peak goes negative (-0.30) once the others are present. The learned router costs slightly more at the same escalation rate because it prefers long, high-variance drafts."
            />
          </Panel>
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          <Panel
            label="Signal Quality (AUROC vs chance, 95% CI)"
            className="lg:col-span-2"
          >
            <AurocChart />
          </Panel>
          <Panel label="Open-Weight Drafter">
            <FactList
              facts={[
                { k: "Drafter", v: "qwen2.5-coder:7b, Ollama" },
                { k: "Hardware", v: "RTX 4050 laptop, 6 GB" },
                { k: "Heavyweight", v: "gpt-4.1, GSM8K r1, n=300" },
                { k: "Drafter alone", v: "83.7% (79.3 to 87.7)" },
                { k: "Router T=2.0", v: "87.3%, escalates 28.0%", color: "var(--color-orange)" },
                { k: "Random, same rate", v: "88.0% (84.3 to 91.7)" },
                { k: "Mean-entropy, same rate", v: "90.3% (86.7 to 93.7)", color: "var(--color-green)" },
                { k: "Heavyweight alone", v: "97.0% (95.0 to 98.7)" },
                { k: "Min-logprob sweep", v: "100% to 0%, no ceiling" },
              ]}
              note="The pattern transfers to a different model family: window peak at chance (0.52), trace mean informative (0.73), router below random. T=2.0 escalates 28.0% of this drafter against 5.3% for gpt-4.1-nano on the same prompts; thresholds do not transfer between drafters. Ollama reports real logprobs for tokens outside the top 5, which confirms the OpenAI -9999 sentinel ceiling is an API encoding artifact."
            />
          </Panel>
        </div>

        <Section
          index="03"
          title="Components Measured by Replay"
          sub="recorded traces fed through the production executor, store and gateway"
        />
        <div className="grid gap-4 lg:grid-cols-5">
          <Panel label="Speculative Execution (soft trigger 0.8T)" className="lg:col-span-3">
            <SpeculativeTable />
          </Panel>
          <Panel label="Semantic Cache (cosine > 0.95)" className="lg:col-span-2">
            <CacheTable />
          </Panel>
        </div>
        <div className="grid gap-4 lg:grid-cols-3">
          <Panel label="Load Test with Fault Injection (vegeta, 50 req/s x 30 s)" className="lg:col-span-2">
            <LoadTestTable />
          </Panel>
          <Panel label="Graceful Degradation">
            <FactList
              facts={[
                { k: "Drafter 5xx or timeout", v: "served by heavyweight", color: "var(--color-green)" },
                { k: "Heavyweight fails after escalation", v: "re-run drafter", color: "var(--color-green)" },
                { k: "Flagged on", v: "response headers + metrics" },
                { k: "Handler tests", v: "9 fallback cases, race-clean" },
                { k: "Flaky run 5xx", v: "0 of 1500" },
                { k: "Before round 1", v: "any provider error failed the request", color: "var(--color-red)" },
              ]}
              note="Both fallbacks are configurable. The mock's uncertain tokens originally carried 1.58 bits, below T=2.0, so earlier load tests never exercised escalation; the mixed scenario now escalates about 21% of requests."
            />
          </Panel>
        </div>

        <Section
          index="04"
          title="Project Set"
          sub="519 self-authored prompts, 4 categories, LLM-as-judge, the source of the headline"
        />
        <div className="grid gap-4 lg:grid-cols-3">
          <Panel label="Cost Reduction by Threshold" className="lg:col-span-2">
            <CostBar />
          </Panel>
          <Panel label="Confusion Matrix at T=2.0">
            <ConfusionMatrix />
            <p className="mt-3 text-[10px] leading-relaxed text-(--color-text-dark)">
              40 escalations caught exactly one bad draft. Random escalation
              at the same rate also caught one. With 11 positives and a judge
              that is the heavyweight itself, this set cannot distinguish
              routers; the answer-key sets above can.
            </p>
          </Panel>
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          <Panel label="Threshold Sweep (n=519)" className="lg:col-span-2">
            <p className="mb-4 text-xs leading-relaxed text-(--color-text-dark)">
              4 categories: factual, reasoning, code generation,
              ambiguous/creative. LLM-as-judge evaluation. Swept T=1.0..2.5 in
              0.25 steps. Top-5 renormalised entropy caps at log2(5) = 2.32
              bits, so the T=2.25 and 2.50 rows are degenerate.
            </p>
            <SweepTable />
          </Panel>
          <Panel label="Cost Model">
            <div className="space-y-4">
              <div>
                <div className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
                  Baseline (all-heavyweight)
                </div>
                <div className="mt-1 font-mono text-xl text-(--color-text)">
                  $1.623
                </div>
                <div className="text-[10px] text-(--color-text-dark)">
                  519 prompts, input + output tokens
                </div>
              </div>
              <div>
                <div className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
                  Routed at T=2.0
                </div>
                <div className="mt-1 font-mono text-xl text-(--color-green)">
                  $0.277
                </div>
                <div className="text-[10px] text-(--color-text-dark)">
                  82.9% reduction, USD 0.534 per 1k requests vs 3.127
                </div>
              </div>
              <div className="space-y-1.5 border-t border-(--color-border) pt-4">
                {[
                  ["DRAFTER OUTPUT", "$0.80/1M tok"],
                  ["HEAVY OUTPUT", "$10.00/1M tok"],
                  ["DRAFTER INPUT", "$0.20/1M tok"],
                  ["HEAVY INPUT", "$2.50/1M tok"],
                ].map(([k, v]) => (
                  <div
                    key={k}
                    className="flex justify-between font-mono text-[10px]"
                  >
                    <span className="text-(--color-text-dark)">{k}</span>
                    <span className="text-(--color-text-dim)">{v}</span>
                  </div>
                ))}
              </div>
              <div className="space-y-1.5 border-t border-(--color-border) pt-4">
                {[
                  ["DRAFTER", "gpt-4.1-nano"],
                  ["HEAVYWEIGHT", "gpt-4.1"],
                  ["EMBEDDINGS", "text-embedding-3-small"],
                ].map(([k, v]) => (
                  <div
                    key={k}
                    className="flex justify-between font-mono text-[10px]"
                  >
                    <span className="text-(--color-text-dark)">{k}</span>
                    <span className="text-(--color-text-dim)">{v}</span>
                  </div>
                ))}
              </div>
            </div>
          </Panel>
        </div>

        <Section
          index="05"
          title="Design"
          sub="what the gateway does and where the measurements bite"
        />
        <div className="grid gap-4 lg:grid-cols-2">
          <Panel label="Entropy Analysis">
            <p className="mb-4 text-xs leading-relaxed text-(--color-text-dark)">
              H(X) = -SUM p(x) log2 p(x) over top-5 logprobs per token. 10-token
              sliding window. The router thresholds the window peak; the
              trace mean is the statistic that carries signal.
            </p>
            <EntropyViz />
          </Panel>
          <Panel label="Routing State Machine">
            <StateMachine />
            <div className="mt-5 border-t border-(--color-border) pt-4">
              <div className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
                Key Parameters
              </div>
              <div className="mt-3 grid grid-cols-1 gap-x-8 gap-y-1.5 font-mono text-[10px] sm:grid-cols-2">
                {[
                  ["HARD THRESHOLD", "T=2.0 bits", "var(--color-orange)"],
                  ["SOFT THRESHOLD", "0.8*T=1.6 bits, per token", "var(--color-amber)"],
                  ["WINDOW", "10 tokens", "var(--color-text-dim)"],
                  ["EARLY EXIT", "10 tokens", "var(--color-text-dim)"],
                  ["CACHE COSINE", ">0.95", "var(--color-cyan)"],
                  ["TOP LOGPROBS", "5 (max 2.32 bits)", "var(--color-text-dim)"],
                ].map(([k, v, c]) => (
                  <div key={k} className="flex justify-between">
                    <span className="text-(--color-text-dark)">{k}</span>
                    <span style={{ color: c }}>{v}</span>
                  </div>
                ))}
              </div>
            </div>
          </Panel>
        </div>

        <div className="grid gap-4 lg:grid-cols-5">
          <Panel label="System Architecture" className="lg:col-span-3">
            <Architecture />
          </Panel>
          <Panel label="Known Tradeoffs" className="lg:col-span-2">
            <div className="space-y-4 text-xs">
              {tradeoffs.map((item, i) => (
                <div
                  key={item.title}
                  className={
                    i > 0 ? "border-t border-(--color-border) pt-4" : ""
                  }
                >
                  <div
                    className="font-mono text-[11px] font-medium"
                    style={{ color: item.color }}
                  >
                    {item.title}
                  </div>
                  <p className="mt-1 leading-relaxed text-(--color-text-dark)">
                    {item.text}
                  </p>
                </div>
              ))}
            </div>
          </Panel>
        </div>

        <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-5">
          {[
            ["GATEWAY", "Go net/http"],
            ["DRAFTER", "gpt-4.1-nano, qwen2.5-coder:7b"],
            ["HEAVYWEIGHT", "gpt-4.1"],
            ["CACHE", "Qdrant + Redis"],
            ["OBS", "Prometheus + Grafana"],
          ].map(([label, value]) => (
            <div
              key={label}
              className="border border-(--color-border) bg-(--color-surface) px-3 py-2.5 sm:px-4"
            >
              <div className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
                {label}
              </div>
              <div className="mt-0.5 text-xs text-(--color-text-dim)">
                {value}
              </div>
            </div>
          ))}
        </div>

        <Panel label="Reproduce">
          <p className="mb-3 text-xs leading-relaxed text-(--color-text-dark)">
            Every number on this page replays offline from traces committed
            under benchmarks/results. No API key is needed for the sweeps,
            the learned-router evaluation or the speculative replay; the
            cache replay makes embedding calls (about one cent per thousand
            prompts). Full commands in docs/RESULTS.md section 14.
          </p>
          <pre className="overflow-x-auto bg-(--color-surface-2) p-3 font-mono text-[10px] leading-relaxed text-(--color-text-dim)">
{`go test -race ./...
go run ./benchmarks/cmd/compare -input benchmarks/results/collected.jsonl.gz -output benchmarks/results/compare.csv
go run ./benchmarks/cmd/compare -input benchmarks/results/collected_gsm8k_r2.jsonl.gz -output benchmarks/results/compare_gsm8k_r2.csv \\
  -models benchmarks/models/router_full_logistic.json
go run ./benchmarks/cmd/replayspec -input benchmarks/results/collected_mmlu_r2.jsonl.gz
OPENAI_API_KEY=... go run ./benchmarks/cmd/replaycache -input benchmarks/results/collected_mmlu.jsonl.gz`}
          </pre>
        </Panel>
      </main>

      <footer className="border-t border-(--color-border)">
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 py-4 text-[11px] text-(--color-text-dark) sm:flex-row sm:px-6">
          <span>
            Built by Anh Tran. Every metric is measured on real APIs or a
            local open model and replayed from committed traces. Inspired by{" "}
            <a
              href="https://arxiv.org/abs/2603.00578"
              target="_blank"
              rel="noopener noreferrer"
              className="text-(--color-text-dim) underline decoration-(--color-border-bright) underline-offset-2 transition-colors hover:text-(--color-text)"
            >
              Draft-Thinking (arXiv:2603.00578)
            </a>
            .
          </span>
          <a
            href="https://github.com/trnahnh/draft-thinker"
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-(--color-cyan) transition-colors hover:text-(--color-text)"
          >
            github.com/trnahnh/draft-thinker
          </a>
        </div>
      </footer>
    </div>
  );
}
