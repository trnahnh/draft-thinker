const data = [
  { t: "1.00", esc: 68.9, acc: 100.0, cost: 8.2, n_acc: 161, n_esc: 357 },
  { t: "1.25", esc: 49.2, acc: 99.6, cost: 31.0, n_acc: 263, n_esc: 255 },
  { t: "1.50", esc: 30.9, acc: 98.6, cost: 56.2, n_acc: 358, n_esc: 160 },
  { t: "1.75", esc: 13.9, acc: 98.4, cost: 81.2, n_acc: 446, n_esc: 72 },
  { t: "2.00", esc: 6.0, acc: 98.2, cost: 91.6, n_acc: 487, n_esc: 31 },
  { t: "2.25", esc: 0.4, acc: 97.9, cost: 99.0, n_acc: 516, n_esc: 2 },
  { t: "2.50", esc: 0.0, acc: 97.9, cost: 99.2, n_acc: 518, n_esc: 0 },
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
