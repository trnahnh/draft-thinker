type MetricProps = {
  value: string;
  label: string;
  sublabel?: string;
  accent?: "orange" | "cyan" | "green" | "red" | "amber" | "default";
  size?: "lg" | "sm";
};

const borderColors = {
  orange: "border-l-[var(--color-orange)]",
  cyan: "border-l-[var(--color-cyan)]",
  green: "border-l-[var(--color-green)]",
  red: "border-l-[var(--color-red)]",
  amber: "border-l-[var(--color-amber)]",
  default: "border-l-[var(--color-border-bright)]",
};

const valueColors = {
  orange: "text-[var(--color-orange)]",
  cyan: "text-[var(--color-cyan)]",
  green: "text-[var(--color-green)]",
  red: "text-[var(--color-red)]",
  amber: "text-[var(--color-amber)]",
  default: "text-[var(--color-text)]",
};

export default function Metric({
  value,
  label,
  sublabel,
  accent = "default",
  size = "lg",
}: MetricProps) {
  return (
    <div
      className={`border border-(--color-border) border-l-2 bg-(--color-surface) px-4 py-4 sm:px-5 sm:py-5 ${borderColors[accent]}`}
    >
      <div className="font-mono text-[10px] uppercase tracking-widest text-(--color-text-dark)">
        {label}
      </div>
      <div
        className={`mt-1.5 font-mono font-medium tracking-tight ${size === "lg" ? "text-2xl sm:text-3xl" : "text-lg sm:text-2xl"} ${valueColors[accent]}`}
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
