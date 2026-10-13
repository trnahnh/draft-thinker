type Score = { v: number; lo: number; hi: number } | null;

type Row = {
  name: string;
  pos: number;
  peak: Score;
  mean: Score;
  learned: Score;
  learnedNote?: string;
};

const rows: Row[] = [
  {
    name: "GSM8K r1",
    pos: 22,
    peak: { v: 0.509, lo: 0.357, hi: 0.649 },
    mean: { v: 0.76, lo: 0.65, hi: 0.858 },
    learned: null,
    learnedNote: "training set",
  },
  {
    name: "MMLU r1",
    pos: 73,
    peak: { v: 0.474, lo: 0.399, hi: 0.551 },
    mean: { v: 0.626, lo: 0.547, hi: 0.698 },
    learned: null,
    learnedNote: "training set",
  },
  {
    name: "GSM8K r2",
    pos: 50,
    peak: { v: 0.569, lo: 0.483, hi: 0.658 },
    mean: { v: 0.805, lo: 0.734, hi: 0.867 },
    learned: { v: 0.808, lo: 0.736, hi: 0.873 },
  },
  {
    name: "ARC-Challenge",
    pos: 37,
    peak: { v: 0.519, lo: 0.423, hi: 0.617 },
    mean: { v: 0.718, lo: 0.645, hi: 0.795 },
    learned: { v: 0.748, lo: 0.665, hi: 0.822 },
  },
  {
    name: "MMLU r2",
    pos: 116,
    peak: { v: 0.445, lo: 0.388, hi: 0.503 },
    mean: { v: 0.695, lo: 0.647, hi: 0.743 },
    learned: { v: 0.715, lo: 0.662, hi: 0.767 },
  },
  {
    name: "GSM8K, qwen",
    pos: 49,
    peak: { v: 0.516, lo: 0.426, hi: 0.606 },
    mean: { v: 0.727, lo: 0.649, hi: 0.798 },
    learned: null,
    learnedNote: "not retrained",
  },
  {
    name: "Project set (judge)",
    pos: 11,
    peak: { v: 0.684, lo: 0.59, hi: 0.779 },
    mean: { v: 0.515, lo: 0.382, hi: 0.643 },
    learned: { v: 0.741, lo: 0.6, hi: 0.876 },
  },
];

const series = [
  { key: "peak", label: "WINDOW PEAK (PRODUCTION)", color: "var(--color-orange)" },
  { key: "mean", label: "MEAN ENTROPY", color: "var(--color-green)" },
  { key: "learned", label: "LEARNED LOGISTIC", color: "var(--color-cyan)" },
] as const;

function Bar({ s, color, note }: { s: Score; color: string; note?: string }) {
  if (!s) {
    return (
      <div className="flex h-2.5 items-center pl-1 font-mono text-[9px] text-(--color-text-dark)">
        {note}
      </div>
    );
  }
  return (
    <div className="relative h-2.5 bg-(--color-surface-2)">
      <div
        className="absolute top-0 h-full opacity-25"
        style={{
          left: `${s.lo * 100}%`,
          width: `${(s.hi - s.lo) * 100}%`,
          backgroundColor: color,
        }}
      />
      <div
        className="absolute top-0 h-full"
        style={{ width: `${s.v * 100}%`, backgroundColor: color }}
      />
    </div>
  );
}

export default function AurocChart() {
  return (
    <div>
      <div className="mb-3 flex flex-wrap gap-x-4 gap-y-1 font-mono text-[10px] text-(--color-text-dark)">
        {series.map((s) => (
          <span key={s.key} className="flex items-center gap-1.5">
            <span className="h-2 w-2" style={{ backgroundColor: s.color }} />
            {s.label}
          </span>
        ))}
        <span className="flex items-center gap-1.5">
          <span className="h-2 w-px bg-(--color-text-dim)" />
          CHANCE 0.5
        </span>
      </div>
      <div className="space-y-3">
        {rows.map((r) => (
          <div key={r.name} className="grid grid-cols-[7.5rem_1fr_2.6rem] gap-x-3 sm:grid-cols-[9rem_1fr_3rem]">
            <div className="pt-px">
              <div className="truncate font-mono text-xs text-(--color-text)">
                {r.name}
              </div>
              <div className="font-mono text-[9px] text-(--color-text-dark)">
                {r.pos} positives
              </div>
            </div>
            <div className="relative space-y-1">
              <div className="pointer-events-none absolute inset-y-0 left-1/2 z-10 w-px bg-(--color-text-dim) opacity-60" />
              {series.map((s) => (
                <Bar
                  key={s.key}
                  s={r[s.key]}
                  color={s.color}
                  note={s.key === "learned" ? r.learnedNote : undefined}
                />
              ))}
            </div>
            <div className="space-y-1 font-mono text-[9px] leading-[10px]">
              {series.map((s) => {
                const v = r[s.key];
                return (
                  <div key={s.key} className="h-2.5" style={{ color: s.color }}>
                    {v ? v.v.toFixed(2) : ""}
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>
      <p className="mt-4 text-[10px] leading-relaxed text-(--color-text-dark)">
        AUROC of each per-request score as a detector of wrong drafts, bar
        at the point estimate, lighter band is the bootstrap 95% interval.
        The window peak is the exact statistic the gateway thresholds. Its
        interval covers 0.5 on all five answer-key samples. Mean entropy is
        informative on every one of them but needs the whole draft.
      </p>
    </div>
  );
}
