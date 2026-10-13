const th =
  "border-b border-(--color-border) pb-2.5 text-right font-normal whitespace-nowrap";
const td = "border-b border-(--color-border) py-2.5 text-right whitespace-nowrap";

const spec = [
  { t: "Project set", n: 519, esc: 40, trig: 70.5, cancel: 96.4, head: "13 of 40", p50: 36, per1k: 680 },
  { t: "GSM8K r1", n: 300, esc: 16, trig: 87.7, cancel: 98.9, head: "3 of 16", p50: 26, per1k: 867 },
  { t: "MMLU r1", n: 300, esc: 56, trig: 86.0, cancel: 93.4, head: "17 of 56", p50: 27, per1k: 803 },
  { t: "GSM8K r2", n: 500, esc: 34, trig: 88.8, cancel: 98.6, head: "6 of 34", p50: 37, per1k: 876 },
  { t: "ARC-Challenge", n: 500, esc: 66, trig: 88.8, cancel: 96.4, head: "16 of 66", p50: 30, per1k: 856 },
  { t: "MMLU r2", n: 500, esc: 95, trig: 86.4, cancel: 92.6, head: "32 of 95", p50: 29, per1k: 800 },
];

export function SpeculativeTable() {
  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full font-mono text-xs">
          <thead>
            <tr className="text-[10px] uppercase tracking-widest text-(--color-text-dark)">
              <th className={`${th} text-left`}>Trace</th>
              <th className={`${th} hidden sm:table-cell`}>Escal</th>
              <th className={`${th} text-(--color-amber)`}>Trigger%</th>
              <th className={`${th} text-(--color-red)`}>Cancel%</th>
              <th className={`${th} hidden md:table-cell`}>Head start</th>
              <th className={th}>p50</th>
              <th className={th}>Cancel/1k</th>
            </tr>
          </thead>
          <tbody>
            {spec.map((r) => (
              <tr key={r.t}>
                <td className={`${td} text-left text-(--color-text)`}>
                  {r.t}
                  <span className="ml-2 text-[10px] text-(--color-text-dark)">
                    n={r.n}
                  </span>
                </td>
                <td className={`${td} hidden text-(--color-text-dark) sm:table-cell`}>
                  {r.esc}
                </td>
                <td className={`${td} text-(--color-amber)`}>{r.trig.toFixed(1)}</td>
                <td className={`${td} text-(--color-red)`}>{r.cancel.toFixed(1)}</td>
                <td className={`${td} hidden text-(--color-text-dark) md:table-cell`}>
                  {r.head}
                </td>
                <td className={`${td} text-(--color-text-dim)`}>{r.p50} ms</td>
                <td className={`${td} text-(--color-text-dim)`}>{r.per1k}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-3 text-[10px] leading-relaxed text-(--color-text-dark)">
        Every recorded trace replayed through the production executor with
        synthetic logprobs that reproduce the recorded entropy exactly.
        Trigger is the share of requests that start a speculative
        heavyweight call at 0.8T; Cancel is the share of those calls
        cancelled because the draft was accepted. Head start is escalations
        that had a speculative call already running. Billed to completion,
        the cancelled calls would cost 80 to 88% of the all-heavyweight
        bill. Recommendation on record: speculative.enabled false.
      </p>
    </div>
  );
}

const cache = [
  { t: "Project set", n: 519, ins: 479, hits: 0, wrong: 0, nn: 0.878, p50: 197, p99: 374 },
  { t: "GSM8K r1", n: 300, ins: 284, hits: 0, wrong: 0, nn: 0.763, p50: 199, p99: 406 },
  { t: "MMLU r1", n: 300, ins: 241, hits: 3, wrong: 1, nn: 1.0, p50: 201, p99: 360 },
];

export function CacheTable() {
  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full font-mono text-xs">
          <thead>
            <tr className="text-[10px] uppercase tracking-widest text-(--color-text-dark)">
              <th className={`${th} text-left`}>Trace</th>
              <th className={`${th} text-(--color-cyan)`}>Hits</th>
              <th className={`${th} text-(--color-red)`}>Wrong</th>
              <th className={th}>NN max</th>
              <th className={th}>p50 / p99</th>
            </tr>
          </thead>
          <tbody>
            {cache.map((r) => (
              <tr key={r.t}>
                <td className={`${td} text-left text-(--color-text)`}>
                  {r.t}
                  <span className="ml-2 text-[10px] text-(--color-text-dark)">
                    n={r.n}
                  </span>
                </td>
                <td className={`${td} text-(--color-cyan)`}>
                  {r.hits}
                  <span className="ml-1 text-[10px] text-(--color-text-dark)">
                    ({((r.hits / r.n) * 100).toFixed(1)}%)
                  </span>
                </td>
                <td className={`${td} ${r.wrong > 0 ? "text-(--color-red)" : "text-(--color-text-dark)"}`}>
                  {r.wrong}
                </td>
                <td className={`${td} text-(--color-text-dim)`}>{r.nn.toFixed(3)}</td>
                <td className={`${td} text-(--color-text-dim)`}>
                  {r.p50} / {r.p99} ms
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="mt-3 space-y-1 text-[10px] leading-relaxed text-(--color-text-dark)">
        <p>
          Prompts replayed in order through the production vector store with
          the real embedding client, inserting what the T=2.0 router would
          have accepted. NN max is the highest cosine any prompt reached
          against an earlier one. The MMLU hits are two exact duplicates and
          one templated near-duplicate at 0.959 that served the wrong letter.
        </p>
        <p>
          At 0.90 the MMLU replay would hit 30 prompts and serve a wrong
          cached answer on 19. A second pass over the project set hits all
          479 inserted prompts. Lookup is dominated by the embedding call.
        </p>
      </div>
    </div>
  );
}

const load = [
  { s: "confident", desc: "all drafts accepted", p50: 105.9, p95: 106.9, p99: 107.4, dec: "accept 1500" },
  { s: "mixed", desc: "20% of drafts uncertain", p50: 106.3, p95: 108.4, p99: 109.8, dec: "accept 1188, escalate 312" },
  { s: "flaky", desc: "20% drafter 503s, 5% stalls, 2 s timeout", p50: 107.1, p95: 111.0, p99: 2107.9, dec: "accept 895, escalate 249, fallback 356" },
];

export function LoadTestTable() {
  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full font-mono text-xs">
          <thead>
            <tr className="text-[10px] uppercase tracking-widest text-(--color-text-dark)">
              <th className={`${th} text-left`}>Scenario</th>
              <th className={th}>Success</th>
              <th className={th}>p50</th>
              <th className={`${th} hidden sm:table-cell`}>p95</th>
              <th className={th}>p99</th>
              <th className={`${th} hidden md:table-cell`}>Decisions</th>
            </tr>
          </thead>
          <tbody>
            {load.map((r) => (
              <tr key={r.s}>
                <td className={`${td} text-left`}>
                  <span className="text-(--color-text)">{r.s}</span>
                  <span className="ml-2 hidden text-[10px] text-(--color-text-dark) sm:inline">
                    {r.desc}
                  </span>
                </td>
                <td className={`${td} text-(--color-green)`}>100%</td>
                <td className={`${td} text-(--color-text-dim)`}>{r.p50.toFixed(1)} ms</td>
                <td className={`${td} hidden text-(--color-text-dim) sm:table-cell`}>
                  {r.p95.toFixed(1)} ms
                </td>
                <td className={`${td} ${r.p99 > 1000 ? "text-(--color-amber)" : "text-(--color-text-dim)"}`}>
                  {r.p99.toFixed(1)} ms
                </td>
                <td className={`${td} hidden text-(--color-text-dark) md:table-cell`}>
                  {r.dec}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-3 text-[10px] leading-relaxed text-(--color-text-dark)">
        1,500 requests per scenario at 50 req/s with vegeta against a mock
        upstream that streams 20 SSE chunks at 5 ms each, so p50 is upstream
        time and gateway overhead is low single-digit milliseconds. In the
        flaky run every injected drafter failure was served by the
        heavyweight with zero 5xx responses. The 2.1 s p99 is the drafter
        timeout plus one heavyweight call. Config: config.loadtest.yaml.
      </p>
    </div>
  );
}
