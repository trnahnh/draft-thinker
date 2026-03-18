type MetricProps = {
  value: string;
  label: string;
  sublabel?: string;
  accent?: "orange" | "cyan" | "green" | "default";
};

const borderColors = {
  orange: "border-l-[var(--color-orange)]",
  cyan: "border-l-[var(--color-cyan)]",
  green: "border-l-[var(--color-green)]",
  default: "border-l-[var(--color-border-bright)]",
};

const valueColors = {
  orange: "text-[var(--color-orange)]",
  cyan: "text-[var(--color-cyan)]",
  green: "text-[var(--color-green)]",
  default: "text-[var(--color-text)]",
};

export default function Metric({
  value,
  label,
  sublabel,
  accent = "default",
}: MetricProps) {
  return (
    <div
      className={`border border-(--color-border) border-l-2 bg-(--color-surface) px-4 py-4 sm:px-5 sm:py-5 ${borderColors[accent]}`}
    >
      <div className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
        {label}
      </div>
      <div
        className={`mt-1.5 font-mono text-2xl font-medium tracking-tight sm:text-3xl ${valueColors[accent]}`}
      >
        {value}
      </div>
      {sublabel && (
        <div className="mt-1 text-[11px] text-(--color-text-dark)">
          {sublabel}
        </div>
      )}
    </div>
  );
}
