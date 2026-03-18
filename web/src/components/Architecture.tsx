function Node({
  label,
  sub,
  accent,
  className = "",
}: {
  label: string;
  sub: string;
  accent?: "orange" | "cyan" | "green" | "red";
  className?: string;
}) {
  const colors = {
    orange: {
      border: "border-l-[var(--color-orange)]",
      text: "text-[var(--color-orange)]",
    },
    cyan: {
      border: "border-l-[var(--color-cyan)]",
      text: "text-[var(--color-cyan)]",
    },
    green: {
      border: "border-l-[var(--color-green)]",
      text: "text-[var(--color-green)]",
    },
    red: {
      border: "border-l-[var(--color-red)]",
      text: "text-[var(--color-red)]",
    },
  };
  const c = accent
    ? colors[accent]
    : {
        border: "border-l-[var(--color-border-bright)]",
        text: "text-[var(--color-text)]",
      };

  return (
    <div
      className={`border border-(--color-border) border-l-2 bg-(--color-surface) px-3 py-2 ${c.border} ${className}`}
    >
      <div className={`font-mono text-[11px] font-medium ${c.text}`}>
        {label}
      </div>
      <div className="mt-0.5 text-[10px] text-(--color-text-dark)">{sub}</div>
    </div>
  );
}

function VLine({
  color = "var(--color-border-bright)",
  height = 20,
  label,
  labelColor,
}: {
  color?: string;
  height?: number;
  label?: string;
  labelColor?: string;
}) {
  return (
    <div className="flex flex-col items-center">
      {label && (
        <div
          className="font-mono text-[9px]"
          style={{ color: labelColor || color }}
        >
          {label}
        </div>
      )}
      <div
        className="w-px opacity-50"
        style={{ backgroundColor: color, height }}
      />
      <div className="text-[7px]" style={{ color }}>
        &#9662;
      </div>
    </div>
  );
}

function HLine() {
  return (
    <div className="flex items-center self-center">
      <div className="h-px w-5 bg-(--color-border-bright) opacity-50 sm:w-8" />
      <div className="text-[7px] text-(--color-border-bright)">&#9656;</div>
    </div>
  );
}

export default function Architecture() {
  return (
    <div className="space-y-1">
      <div className="flex flex-col items-start gap-1 sm:flex-row sm:items-stretch sm:gap-0">
        <Node label="CLIENT" sub="HTTP POST" />
        <HLine />
        <Node label="GATEWAY" sub="Go net/http, goroutines" />
        <HLine />
        <Node
          label="CACHE"
          sub="Qdrant + Redis, cosine &gt; 0.95"
          accent="cyan"
        />
      </div>

      <div className="flex gap-12 pl-4 sm:pl-[200px]">
        <div className="flex flex-col items-center">
          <VLine color="var(--color-green)" label="HIT" height={16} />
          <div className="border border-(--color-green) border-opacity-30 bg-(--color-green-dim) px-3 py-1.5">
            <div className="font-mono text-[10px] text-(--color-green)">
              RETURN CACHED
            </div>
          </div>
        </div>
        <div className="flex flex-col items-center">
          <VLine label="MISS" height={16} />
        </div>
      </div>

      <div className="flex flex-col items-start gap-1 pl-4 sm:flex-row sm:items-stretch sm:gap-0 sm:pl-[104px]">
        <Node
          label="ENTROPY ENGINE"
          sub="H(X) = -SUM p log2 p, 10-tok window"
        />
        <HLine />
        <Node
          label="DRAFTER"
          sub="gpt-4.1-nano, logprobs top-5"
          accent="green"
        />
      </div>

      <div className="flex justify-start gap-14 pl-10 sm:gap-20 sm:pl-[140px]">
        <div className="flex flex-col items-center">
          <VLine
            color="var(--color-green)"
            label="H(X) < T"
            labelColor="var(--color-green)"
            height={16}
          />
          <Node label="ACCEPT" sub="serve draft, insert cache" accent="green" />
        </div>
        <div className="flex flex-col items-center">
          <VLine
            color="var(--color-orange)"
            label="H(X) >= T"
            labelColor="var(--color-orange)"
            height={16}
          />
          <Node label="ESCALATE" sub="route to heavyweight" accent="orange" />
          <VLine color="var(--color-orange)" height={12} />
          <Node label="HEAVYWEIGHT" sub="gpt-4.1" accent="red" />
          <div className="mt-1 font-mono text-[9px] text-(--color-amber)">
            speculate at 0.8*T via goroutine
          </div>
        </div>
      </div>

      <div className="flex justify-center pt-1">
        <VLine height={12} />
      </div>
      <div className="flex justify-center">
        <Node
          label="PROMETHEUS"
          sub="11 instruments, Grafana 12-panel dashboard"
        />
      </div>
    </div>
  );
}
