const data = [
  { t: 1.0, cost: 8.2, acc: 100.0 },
  { t: 1.25, cost: 31.0, acc: 99.6 },
  { t: 1.5, cost: 56.2, acc: 98.6 },
  { t: 1.75, cost: 81.2, acc: 98.4 },
  { t: 2.0, cost: 91.6, acc: 98.2 },
  { t: 2.25, cost: 99.0, acc: 97.9 },
  { t: 2.5, cost: 99.2, acc: 97.9 },
];

export default function CostBar() {
  return (
    <div className="space-y-1.5">
      <div className="mb-3 flex font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
        <span className="w-14 shrink-0">T</span>
        <span className="flex-1">COST REDUCTION</span>
        <span className="w-14 text-right">ACC</span>
      </div>
      {data.map((d) => {
        const sel = d.t === 2.0;
        return (
          <div key={d.t} className="flex items-center">
            <span
              className={`w-14 shrink-0 font-mono text-xs ${
                sel ? "text-(--color-orange)" : "text-(--color-text-dark)"
              }`}
            >
              {d.t.toFixed(2)}
            </span>
            <div className="relative h-6 flex-1 bg-(--color-surface-2)">
              <div
                className={`h-full transition-all ${sel ? "bg-(--color-orange)" : "bg-(--color-border-bright)"}`}
                style={{ width: `${d.cost}%` }}
              />
              {sel && (
                <span className="absolute left-2 top-1/2 -translate-y-1/2 font-mono text-[10px] font-medium text-black">
                  {d.cost}%
                </span>
              )}
            </div>
            <span
              className={`w-14 text-right font-mono text-xs ${
                d.acc >= 98
                  ? "text-(--color-green)"
                  : "text-(--color-text-dark)"
              }`}
            >
              {d.acc}%
            </span>
          </div>
        );
      })}
    </div>
  );
}
