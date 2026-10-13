type Row = {
  name: string;
  n: number;
  labels: string;
  esc: number;
  drafter: number;
  router: number;
  random: number;
  mean: number;
  learned: number | null;
  learnedNote?: string;
  heavy: number;
  heavyNote?: string;
};

const rows: Row[] = [
  {
    name: "GSM8K r1",
    n: 300,
    labels: "answer key",
    esc: 5.3,
    drafter: 92.7,
    router: 93.0,
    random: 92.7,
    mean: 94.3,
    learned: null,
    learnedNote: "train",
    heavy: 95.3,
  },
  {
    name: "MMLU r1",
    n: 300,
    labels: "answer key",
    esc: 18.7,
    drafter: 75.7,
    router: 77.3,
    random: 76.7,
    mean: 79.3,
    learned: null,
    learnedNote: "train",
    heavy: 89.7,
  },
  {
    name: "GSM8K r2",
    n: 500,
    labels: "held out",
    esc: 6.8,
    drafter: 90.0,
    router: 90.4,
    random: 90.8,
    mean: 92.2,
    learned: 92.4,
    heavy: 95.8,
  },
  {
    name: "ARC-Challenge",
    n: 500,
    labels: "held out",
    esc: 13.2,
    drafter: 92.6,
    router: 93.0,
    random: 93.2,
    mean: 93.4,
    learned: 94.0,
    heavy: 96.4,
  },
  {
    name: "MMLU r2",
    n: 500,
    labels: "held out",
    esc: 19.0,
    drafter: 76.8,
    router: 78.2,
    random: 79.2,
    mean: 81.6,
    learned: 82.2,
    heavy: 90.8,
  },
  {
    name: "GSM8K, qwen drafter",
    n: 300,
    labels: "answer key",
    esc: 28.0,
    drafter: 83.7,
    router: 87.3,
    random: 88.0,
    mean: 90.3,
    learned: null,
    learnedNote: "n/a",
    heavy: 97.0,
  },
  {
    name: "Project set",
    n: 519,
    labels: "LLM judge",
    esc: 7.7,
    drafter: 97.9,
    router: 98.1,
    random: 98.1,
    mean: 97.9,
    learned: 98.3,
    heavy: 100.0,
    heavyNote: "def.",
  },
];

const th =
  "border-b border-(--color-border) pb-2.5 text-right font-normal whitespace-nowrap";
const td = "border-b border-(--color-border) py-2.5 text-right whitespace-nowrap";

function delta(a: number, b: number) {
  const d = a - b;
  const s = d > 0 ? "+" : "";
  return `${s}${d.toFixed(1)}`;
}

function deltaColor(a: number, b: number) {
  const d = Math.round((a - b) * 10);
  if (d > 0) return "text-(--color-green)";
  if (d < 0) return "text-(--color-red)";
  return "text-(--color-text-dark)";
}

export default function BenchmarkTable() {
  return (
    <div className="overflow-x-auto">
      <table className="w-full font-mono text-xs">
        <thead>
          <tr className="text-[10px] uppercase tracking-widest text-(--color-text-dark)">
            <th className={`${th} text-left`}>Dataset</th>
            <th className={`${th} hidden sm:table-cell`}>N</th>
            <th className={th}>Esc%</th>
            <th className={`${th} hidden md:table-cell`}>Drafter</th>
            <th className={`${th} text-(--color-orange)`}>Router</th>
            <th className={th}>Random</th>
            <th className={th}>vs Rand</th>
            <th className={th}>Mean-H</th>
            <th className={`${th} text-(--color-cyan)`}>Learned</th>
            <th className={`${th} hidden md:table-cell`}>Heavy</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name}>
              <td className={`${td} text-left`}>
                <span className="text-(--color-text)">{r.name}</span>
                <span className="ml-2 text-[10px] text-(--color-text-dark)">
                  {r.labels}
                </span>
              </td>
              <td className={`${td} hidden text-(--color-text-dark) sm:table-cell`}>
                {r.n}
              </td>
              <td className={`${td} text-(--color-text-dark)`}>
                {r.esc.toFixed(1)}
              </td>
              <td className={`${td} hidden text-(--color-text-dark) md:table-cell`}>
                {r.drafter.toFixed(1)}
              </td>
              <td className={`${td} text-(--color-orange)`}>
                {r.router.toFixed(1)}
              </td>
              <td className={`${td} text-(--color-text-dim)`}>
                {r.random.toFixed(1)}
              </td>
              <td className={`${td} ${deltaColor(r.router, r.random)}`}>
                {delta(r.router, r.random)}
              </td>
              <td className={`${td} text-(--color-text-dim)`}>
                {r.mean.toFixed(1)}
              </td>
              <td className={`${td} text-(--color-cyan)`}>
                {r.learned === null ? (
                  <span className="text-(--color-text-dark)">
                    {r.learnedNote}
                  </span>
                ) : (
                  r.learned.toFixed(1)
                )}
              </td>
              <td className={`${td} hidden text-(--color-text-dark) md:table-cell`}>
                {r.heavy.toFixed(1)}
                {r.heavyNote && (
                  <span className="ml-1 text-[9px]">{r.heavyNote}</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="mt-3 space-y-1 text-[10px] leading-relaxed text-(--color-text-dark)">
        <p>
          Served accuracy in percent. Every method escalates the same share
          of requests as the production router at T=2.0 on that dataset.
          Random is the rate-matched random router. Mean-H thresholds the
          trace mean entropy. Learned is a 22-feature logistic model trained
          on the two r1 sets and tested on the r2 sets and ARC.
        </p>
        <p>
          qwen row: drafter is qwen2.5-coder:7b via local Ollama, heavyweight
          gpt-4.1, T=2.0 not recalibrated. Project set is judged by gpt-4.1
          against its own answer, so the heavyweight is correct by
          definition. Bootstrap 95% intervals for every cell are in
          docs/RESULTS.md sections 3 to 5, 9 and 13.
        </p>
      </div>
    </div>
  );
}
