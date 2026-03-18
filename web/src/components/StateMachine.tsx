const states = [
  {
    name: "DRAFTING",
    cond: "entry",
    desc: "Drafter generates. Entropy computed per token, 10-token sliding window.",
    color: "var(--color-cyan)",
  },
  {
    name: "SPECULATING",
    cond: "H(X) > 0.8*T",
    desc: "Soft threshold crossed. Heavyweight fires in parallel via goroutine.",
    color: "var(--color-amber)",
  },
  {
    name: "ACCEPTED",
    cond: "H(X) < T at EOS",
    desc: "Draft served. Response cached. Speculative heavyweight canceled if running.",
    color: "var(--color-green)",
  },
  {
    name: "ESCALATED",
    cond: "H(X) >= T",
    desc: "Heavyweight response used. Speculative head start reduces tail latency.",
    color: "var(--color-red)",
  },
];

export default function StateMachine() {
  return (
    <div className="space-y-0">
      {states.map((s) => (
        <div
          key={s.name}
          className="flex gap-3 border-b border-(--color-border) py-3 last:border-b-0 sm:gap-4"
        >
          <div className="flex w-6 shrink-0 items-start justify-center pt-1">
            <div className="h-2 w-2" style={{ backgroundColor: s.color }} />
          </div>
          <div className="min-w-0">
            <div className="flex items-baseline gap-2">
              <span className="font-mono text-xs" style={{ color: s.color }}>
                {s.name}
              </span>
              <span className="font-mono text-[10px] text-(--color-text-dark)">
                {s.cond}
              </span>
            </div>
            <div className="mt-0.5 text-xs leading-relaxed text-(--color-text-dim)">
              {s.desc}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
