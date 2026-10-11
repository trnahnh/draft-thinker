import Metric from "@/components/Metric";
import SweepTable from "@/components/SweepTable";
import CostBar from "@/components/CostBar";
import ConfusionMatrix from "@/components/ConfusionMatrix";
import Architecture from "@/components/Architecture";
import EntropyViz from "@/components/EntropyViz";
import StateMachine from "@/components/StateMachine";

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
              519 CALIBRATED
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
            sublabel="95% CI 79.6 to 86.0, vs all-heavyweight"
            accent="orange"
          />
          <Metric
            value="92.3%"
            label="Draft Acceptance"
            sublabel="479 of 519 prompts"
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

        <div className="grid gap-4 lg:grid-cols-3">
          <Panel label="Cost Reduction by Threshold" className="lg:col-span-2">
            <CostBar />
          </Panel>
          <Panel label="Confusion Matrix at T=2.0">
            <ConfusionMatrix />
          </Panel>
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          <Panel label="Threshold Sweep (n=519)" className="lg:col-span-2">
            <p className="mb-4 text-xs leading-relaxed text-(--color-text-dark)">
              4 categories: factual, reasoning, code generation,
              ambiguous/creative. LLM-as-judge evaluation. Swept T=1.0..2.5 in
              0.25 steps.
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
                  82.9% reduction
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

        <div className="grid gap-4 lg:grid-cols-2">
          <Panel label="Entropy Analysis">
            <p className="mb-4 text-xs leading-relaxed text-(--color-text-dark)">
              H(X) = -SUM p(x) log2 p(x) over top-5 logprobs per token. 10-token
              sliding window.
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
                  ["SOFT THRESHOLD", "0.8*T=1.6 bits", "var(--color-amber)"],
                  ["WINDOW", "10 tokens", "var(--color-text-dim)"],
                  ["EARLY EXIT", "10 tokens", "var(--color-text-dim)"],
                  ["CACHE COSINE", ">0.95", "var(--color-cyan)"],
                  ["TOP LOGPROBS", "5", "var(--color-text-dim)"],
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
              {[
                {
                  title: "CONFIDENT HALLUCINATION",
                  color: "var(--color-orange)",
                  text: "Drafter produces wrong answers with low entropy. 10 of 11 bad drafts pass at T=2.0 in 519 prompts. All 11 have mean entropy under 0.5 bits.",
                },
                {
                  title: "ROUTER VS RANDOM",
                  color: "var(--color-red)",
                  text: "A random router escalating the same 7.9% reaches 98.1% accuracy at 83.9% cost reduction. The savings come from the cheap drafter, not from the entropy signal. See docs/RESULTS.md.",
                },
                {
                  title: "SPECULATIVE WASTE",
                  color: "var(--color-amber)",
                  text: "Measured by replaying traces through the executor: fires on 70 to 89% of requests, cancels 93 to 99% of those calls, median head start 26 to 37 ms. The under-10%-waste target is not met. See docs/RESULTS.md section 11.",
                },
                {
                  title: "CACHE CONSERVATISM",
                  color: "var(--color-cyan)",
                  text: "Cosine > 0.95, only draft-accepted responses cached. Replayed: 0 to 1% hits on unique benchmark prompts, 100% on repeats, lookup about 200 ms. At 0.90 MMLU would serve 19 wrong cached answers in 30 hits.",
                },
                {
                  title: "ENTROPY vs CLASSIFICATION",
                  color: "var(--color-green)",
                  text: "Classifiers operate on the prompt. Entropy operates on the generation. Robust to distribution shift across all 4 categories.",
                },
              ].map((item, i) => (
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
            ["DRAFTER", "gpt-4.1-nano"],
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
      </main>

      <footer className="border-t border-(--color-border)">
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 py-4 text-[11px] text-(--color-text-dark) sm:flex-row sm:px-6">
          <span>
            Built by Anh Tran. All metrics measured on real OpenAI APIs.
            Inspired by{" "}
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
