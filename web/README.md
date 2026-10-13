# Draft-Thinker Research Showcase

Static Next.js site presenting Draft-Thinker's calibration results, the held-out evaluation on GSM8K, MMLU and ARC-Challenge, the learned router, the replay measurements of speculative execution and the semantic cache, and the architecture.

Every number on the page is hard-coded from `docs/RESULTS.md`, `ENHANCEMENT_REPORT.md` and `ROUND2_REPORT.md` at the repository root. When those change, update the data arrays in `src/components/` and the metric cards in `src/app/page.tsx` to match; the site has no data fetching.

## Run

```bash
npm install
npm run dev
```

Open http://localhost:3000.

## Build

```bash
npm run build
```

Output in `.next/`. Fully static, no server-side data fetching.
