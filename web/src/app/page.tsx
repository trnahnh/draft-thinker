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
      className={`border border-[var(--color-border)] bg-[var(--color-surface)] ${className}`}
    >
      <div className="border-b border-[var(--color-border)] px-4 py-2.5 sm:px-5">
        <span className="font-mono text-[10px] uppercase tracking-[0.15em] text-[var(--color-text-dark)]">
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
      <nav className="sticky top-0 z-50 border-b border-[var(--color-border)] bg-[var(--color-bg)]/90 backdrop-blur-sm">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6">
          <div className="flex items-center gap-3">
            <span className="font-mono text-sm font-medium tracking-tight text-[var(--color-orange)]">
              DRAFT-THINKER
            </span>
            <span className="hidden font-mono text-[10px] text-[var(--color-text-dark)] sm:inline">
              COST-AWARE LLM GATEWAY
            </span>
          </div>
          <div className="flex items-center gap-3 font-mono text-[10px] text-[var(--color-text-dark)] sm:gap-5">
            <span className="hidden items-center gap-1.5 sm:flex">
              <span className="h-1.5 w-1.5 bg-[var(--color-green)]" />
              518 CALIBRATED
            </span>
            <span className="hidden sm:inline">T=2.0</span>
            <a
              href="https://github.com/trnahnh/draft-thinker"
              target="_blank"
              rel="noopener noreferrer"
              className="text-[var(--color-cyan)] transition-colors hover:text-[var(--color-text)]"
            >
              SOURCE
            </a>
          </div>
        </div>
      </nav>

      <main className="mx-auto max-w-7xl space-y-4 px-4 py-6 sm:px-6 sm:py-8">
        <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
          <Metric
            value="91.6%"
            label="Cost Reduction"
            sublabel="vs all-heavyweight baseline"
            accent="orange"
          />
          <Metric
            value="94.0%"
            label="Draft Acceptance"
            sublabel="487 of 518 prompts"
            accent="green"
          />
          <Metric
            value="98.2%"
            label="Draft Accuracy"
            sublabel="LLM-as-judge eval"
            accent="cyan"
          />
          <Metric
            value="109ms"
            label="P99 Latency"
            sublabel="draft path, 50 req/s"
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
          <Panel label="Threshold Sweep (n=518)" className="lg:col-span-2">
            <p className="mb-4 text-xs leading-relaxed text-[var(--color-text-dark)]">
              4 categories: factual, reasoning, code generation,
              ambiguous/creative. LLM-as-judge evaluation. Swept T=1.0..2.5 in
              0.25 steps.
            </p>
            <SweepTable />
          </Panel>
          <Panel label="Cost Model">
            <div className="space-y-4">
              <div>
                <div className="font-mono text-[10px] uppercase tracking-widest text-[var(--color-text-dark)]">
                  Baseline (all-heavyweight)
                </div>
                <div className="mt-1 font-mono text-xl text-[var(--color-text)]">
                  $1.591
                </div>
                <div className="text-[10px] text-[var(--color-text-dark)]">
                  518 prompts
                </div>
              </div>
              <div>
                <div className="font-mono text-[10px] uppercase tracking-widest text-[var(--color-text-dark)]">
                  Routed at T=2.0
                </div>
                <div className="mt-1 font-mono text-xl text-[var(--color-green)]">
                  $0.133
                </div>
                <div className="text-[10px] text-[var(--color-text-dark)]">
                  91.6% reduction
                </div>
              </div>
              <div className="space-y-1.5 border-t border-[var(--color-border)] pt-4">
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
                    <span className="text-[var(--color-text-dark)]">{k}</span>
                    <span className="text-[var(--color-text-dim)]">{v}</span>
                  </div>
                ))}
              </div>
              <div className="space-y-1.5 border-t border-[var(--color-border)] pt-4">
                {[
                  ["DRAFTER", "gpt-4.1-nano"],
                  ["HEAVYWEIGHT", "gpt-4.1"],
                  ["EMBEDDINGS", "text-embedding-3-small"],
                ].map(([k, v]) => (
                  <div
                    key={k}
                    className="flex justify-between font-mono text-[10px]"
                  >
                    <span className="text-[var(--color-text-dark)]">{k}</span>
                    <span className="text-[var(--color-text-dim)]">{v}</span>
                  </div>
                ))}
              </div>
            </div>
          </Panel>
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <Panel label="Entropy Analysis">
            <p className="mb-4 text-xs leading-relaxed text-[var(--color-text-dark)]">
              H(X) = -SUM p(x) log2 p(x) over top-5 logprobs per token.
              10-token sliding window.
            </p>
            <EntropyViz />
          </Panel>
          <Panel label="Routing State Machine">
            <StateMachine />
            <div className="mt-5 border-t border-[var(--color-border)] pt-4">
              <div className="font-mono text-[10px] uppercase tracking-widest text-[var(--color-text-dark)]">
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
                    <span className="text-[var(--color-text-dark)]">{k}</span>
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
                  text: "Drafter produces wrong answers with low entropy. 9 FN in 518 prompts. Mitigated by conservative T, periodic audits, downstream feedback.",
                },
                {
                  title: "SPECULATIVE WASTE",
                  color: "var(--color-amber)",
                  text: "Heavyweight fires at 0.8*T but drafter recovers. Wasted call. Under 10% of escalation cost. Latency savings on true escalations justify overhead.",
                },
                {
                  title: "CACHE CONSERVATISM",
                  color: "var(--color-cyan)",
                  text: "Cosine > 0.95 strict by design. Only draft-accepted responses cached. Escalated responses excluded. Re-draft beats stale answer.",
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
                    i > 0
                      ? "border-t border-[var(--color-border)] pt-4"
                      : ""
                  }
                >
                  <div
                    className="font-mono text-[11px] font-medium"
                    style={{ color: item.color }}
                  >
                    {item.title}
                  </div>
                  <p className="mt-1 leading-relaxed text-[var(--color-text-dark)]">
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
              className="border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2.5 sm:px-4"
            >
              <div className="font-mono text-[10px] uppercase tracking-widest text-[var(--color-text-dark)]">
                {label}
              </div>
              <div className="mt-0.5 text-xs text-[var(--color-text-dim)]">
                {value}
              </div>
            </div>
          ))}
        </div>
      </main>

      <footer className="border-t border-[var(--color-border)]">
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 py-4 text-[11px] text-[var(--color-text-dark)] sm:flex-row sm:px-6">
          <span>
            Built by Anh Tran. All metrics measured on real OpenAI APIs.
          </span>
          <a
            href="https://github.com/trnahnh/draft-thinker"
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-[var(--color-cyan)] transition-colors hover:text-[var(--color-text)]"
          >
            github.com/trnahnh/draft-thinker
          </a>
        </div>
      </footer>
    </div>
  );
}
