const examples = [
  {
    label: "ARITHMETIC",
    prompt: "What is 347 + 892?",
    mean: 0.028,
    max: 0.303,
    decision: "ACCEPT" as const,
  },
  {
    label: "FACTUAL",
    prompt: "What is photosynthesis?",
    mean: 0.21,
    max: 1.835,
    decision: "ACCEPT" as const,
  },
  {
    label: "AMBIGUOUS",
    prompt: "Define ubiquitous",
    mean: 0.359,
    max: 2.198,
    decision: "ESCALATE" as const,
  },
];

const T = 2.0;
const MAX = 3.0;

export default function EntropyViz() {
  return (
    <div className="space-y-4">
      {examples.map((ex) => (
        <div key={ex.prompt}>
          <div className="mb-1.5 flex items-baseline justify-between gap-2">
            <div className="min-w-0">
              <span className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
                {ex.label}
              </span>
              <span className="ml-2 text-xs text-(--color-text-dim) max-sm:hidden">
                {ex.prompt}
              </span>
            </div>
            <span
              className={`shrink-0 font-mono px-1.5 py-0.5 text-[10px] font-medium ${
                ex.decision === "ACCEPT"
                  ? "bg-(--color-green-dim) text-(--color-green)"
                  : "bg-(--color-orange-dim) text-(--color-orange)"
              }`}
            >
              {ex.decision}
            </span>
          </div>
          <div className="relative h-7 bg-(--color-surface-2)">
            <div
              className="absolute left-0 top-0 h-full bg-(--color-cyan-dim)"
              style={{ width: `${(ex.mean / MAX) * 100}%` }}
            />
            <div
              className={`absolute top-0 h-full w-0.5 ${
                ex.max >= T ? "bg-(--color-orange)" : "bg-(--color-green)"
              }`}
              style={{ left: `${(ex.max / MAX) * 100}%` }}
            />
            <div
              className="absolute top-0 h-full w-px bg-(--color-red) opacity-50"
              style={{ left: `${(T / MAX) * 100}%` }}
            />
            <div className="absolute right-2 top-1/2 -translate-y-1/2 flex gap-3 font-mono text-[10px]">
              <span className="text-(--color-text-dark)">
                avg {ex.mean.toFixed(3)}
              </span>
              <span className="text-(--color-text-dim)">
                peak {ex.max.toFixed(3)}
              </span>
            </div>
          </div>
        </div>
      ))}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 pt-1 font-mono text-[10px] text-(--color-text-dark)">
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-2 bg-(--color-red) opacity-50" />{" "}
          T=2.0
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-2 bg-(--color-cyan-dim)" /> mean
          entropy
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-0.5 bg-(--color-green)" /> peak
          (under T)
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-0.5 bg-(--color-orange)" /> peak
          (over T)
        </span>
      </div>
    </div>
  );
}
