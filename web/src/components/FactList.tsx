type Fact = { k: string; v: string; color?: string };

export default function FactList({
  facts,
  note,
}: {
  facts: Fact[];
  note?: string;
}) {
  return (
    <div>
      <div className="space-y-2.5">
        {facts.map((f) => (
          <div
            key={f.k}
            className="flex flex-col gap-1 border-b border-(--color-border) pb-2.5 last:border-b-0 last:pb-0 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4"
          >
            <span className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
              {f.k}
            </span>
            <span
              className="font-mono text-xs sm:text-right"
              style={{ color: f.color ?? "var(--color-text-dim)" }}
            >
              {f.v}
            </span>
          </div>
        ))}
      </div>
      {note && (
        <p className="mt-4 text-[10px] leading-relaxed text-(--color-text-dark)">
          {note}
        </p>
      )}
    </div>
  );
}
