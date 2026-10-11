const data = [
  { t: "1.00", esc: 68.0, acc: 100.0, cost: 3.6, n_acc: 166, n_esc: 353 },
  { t: "1.25", esc: 48.5, acc: 99.8, cost: 24.4, n_acc: 267, n_esc: 252 },
  { t: "1.50", esc: 32.6, acc: 99.0, cost: 46.4, n_acc: 350, n_esc: 169 },
  { t: "1.75", esc: 15.6, acc: 98.3, cost: 73.2, n_acc: 438, n_esc: 81 },
  { t: "2.00", esc: 7.7, acc: 98.1, cost: 82.9, n_acc: 479, n_esc: 40 },
  { t: "2.25", esc: 0.8, acc: 97.9, cost: 91.9, n_acc: 515, n_esc: 4 },
  { t: "2.50", esc: 0.0, acc: 97.9, cost: 92.6, n_acc: 519, n_esc: 0 },
];

export default function SweepTable() {
  return (
    <div className="overflow-x-auto">
      <table className="w-full font-mono text-xs">
        <thead>
          <tr className="text-[10px] uppercase tracking-widest text-(--color-text-dark)">
            <th className="border-b border-(--color-border) pb-2.5 text-left font-normal">
              T
            </th>
            <th className="border-b border-(--color-border) pb-2.5 text-right font-normal">
              ESC%
            </th>
            <th className="border-b border-(--color-border) pb-2.5 text-right font-normal">
              ACC%
            </th>
            <th className="border-b border-(--color-border) pb-2.5 text-right font-normal">
              COST RED%
            </th>
            <th className="hidden border-b border-(--color-border) pb-2.5 text-right font-normal sm:table-cell">
              ACCEPT
            </th>
            <th className="hidden border-b border-(--color-border) pb-2.5 text-right font-normal sm:table-cell">
              ESCAL
            </th>
          </tr>
        </thead>
        <tbody>
          {data.map((r) => {
            const sel = r.t === "2.00";
            return (
              <tr key={r.t} className={sel ? "bg-(--color-orange-dim)" : ""}>
                <td className="border-b border-(--color-border) py-2.5 text-left">
                  <span
                    className={
                      sel ? "text-(--color-orange)" : "text-(--color-text-dim)"
                    }
                  >
                    {r.t}
                  </span>
                  {sel && (
                    <span className="ml-2 bg-(--color-orange) px-1 py-px text-[9px] font-medium text-black">
                      SEL
                    </span>
                  )}
                </td>
                <td className="border-b border-(--color-border) py-2.5 text-right text-(--color-text-dark)">
                  {r.esc.toFixed(1)}
                </td>
                <td
                  className={`border-b border-(--color-border) py-2.5 text-right ${r.acc >= 98 ? "text-(--color-green)" : "text-(--color-text-dark)"}`}
                >
                  {r.acc.toFixed(1)}
                </td>
                <td
                  className={`border-b border-(--color-border) py-2.5 text-right ${sel ? "text-(--color-orange)" : "text-(--color-text)"}`}
                >
                  {r.cost.toFixed(1)}
                </td>
                <td className="hidden border-b border-(--color-border) py-2.5 text-right text-(--color-text-dark) sm:table-cell">
                  {r.n_acc}
                </td>
                <td className="hidden border-b border-(--color-border) py-2.5 text-right text-(--color-text-dark) sm:table-cell">
                  {r.n_esc}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
