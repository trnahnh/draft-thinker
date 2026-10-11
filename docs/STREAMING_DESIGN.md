# Streaming accepted drafts to the client: design note

Status: design only, nothing implemented. This document exists because the
round 1 audit found that the gateway's latency story ("2 to 5x better time
to first token") was not supported by its own data, and because the fix is
not a code change but a product-contract change that needs decisions
first.

## 1. What the gateway does today

For a streaming request the handler consumes the whole drafter stream,
running the entropy window over every token, before writing a single byte
to the client. If the window never crosses T the buffered draft is replayed
to the client as SSE chunks; if it crosses T the draft is discarded and the
heavyweight stream is forwarded instead. Speculative execution starts the
heavyweight call early but changes nothing about when the client first sees
output.

Consequences, measured on the committed traces (`compare*.json`,
`speculative_*.json`):

- Time to first byte for an accepted draft equals the full drafter
  generation time: p50 3.6 s on the project set at about 290 output tokens.
  The heavyweight alone would deliver its first token in well under a
  second. The gateway is therefore slower to first token than the model it
  is meant to replace, on every request.
- The decision is safe. The client never sees a draft that is later
  retracted, so a wrong draft costs nothing but the drafter call.
- The speculative head start is small. Replayed through the production
  executor at the production soft multiplier (0.8), the median gap between
  the soft trigger and the hard escalation is 36 ms on the project set,
  while 70% of requests trigger a speculative call and 96% of those are
  cancelled. See `docs/RESULTS.md` section 10.

## 2. What streaming the draft would change

Forward each drafter chunk to the client as it arrives, while the entropy
window runs alongside. Two regimes:

1. **Commit after prefix.** Hold the first k tokens (the early-exit window)
   and release them only if no token in the prefix crossed T. After that
   point the gateway is committed to the draft: later window crossings can
   no longer trigger a client-visible switch. Time to first token becomes
   the drafter's time to k tokens (tens to a few hundred milliseconds)
   instead of its time to completion.
2. **Switch mid-stream.** Keep forwarding and, on a late crossing, stop the
   drafter stream and continue with the heavyweight. The client has
   already received a partial answer that the heavyweight did not write,
   so the response is a splice of two models.

Regime 1 is honest and simple but means the router can only act on the
prefix, where every measurement so far says the signal is at chance
(`entropy_window_peak` AUROC 0.51 to 0.57 on GSM8K, 0.47 on MMLU, 0.52 on
ARC; prefix-only learned routers at 0.49 on ARC). Streaming the draft would
turn the gateway into "always drafter, with time-to-first-token" unless a
prefix signal is found.

Regime 2 keeps the whole-trace signal (mean entropy, AUROC 0.71 to 0.81)
but changes what a response is. It needs a protocol for telling the client
that earlier content is superseded, and no OpenAI-compatible client
understands such a signal today.

## 3. Decisions needed before implementation

| # | Decision | Options | What it depends on |
|---|---|---|---|
| 1 | Is a retracted or spliced answer acceptable to any consumer? | Never (regime 1 only); yes with an explicit marker (regime 2, custom SSE event such as `event: retract`); yes silently (splice, not recommended) | Who the clients are; whether the product promises "one model's answer" |
| 2 | Commit point for regime 1 | k = early-exit count (10 tokens, current window); a longer prefix (50 tokens) traded against time to first token; a learned prefix model if one ever beats chance | Prefix AUROC measurements; latency budget |
| 3 | What the client is told | Nothing (looks like a normal stream); `X-Routing-Decision: draft_streamed` on the response headers (headers are sent before the decision is final, so the value would have to be provisional); a trailing SSE event with the final decision | Observability needs; whether headers can be provisional |
| 4 | Behaviour on heavyweight failure after a mid-stream switch | Resume the drafter stream from where it stopped (requires keeping it open, costs drafter tokens); end the stream with an error event; fall back to re-running the drafter and replacing content (regime 2 marker again) | Decision 1 |
| 5 | Cache semantics | Only cache drafts that were streamed to completion without a switch; never cache spliced responses | Decision 1 |
| 6 | Speculative execution | Keep it (it still helps a mid-stream switch start sooner) or remove it (its measured cancel ratio is 93 to 99% and its waste bound exceeds the cost saving) | `speculative_*.json`; whether regime 2 is chosen at all |
| 7 | Non-streaming requests | Unchanged (buffer, decide, respond), since there is no first-token win to have | None |
| 8 | Metrics | Time to first byte histogram split by decision; count of retractions if regime 2; drafter tokens discarded after a switch | Decision 1 and 3 |

## 4. What would have to be measured

- Time to first byte for accepted drafts, before and after, from the load
  test with the mock upstream (`benchmarks/cmd/loadtest`) and from recorded
  traces (drafter time to k tokens is derivable from recorded per-token
  latency only if the collector records chunk arrival times, which it does
  not today; that is a collector change).
- Served accuracy under regime 1, which is the always-drafter accuracy plus
  whatever the prefix catches: 90.0% to 92.9% on the round 2 answer-key
  sets against 95.8% to 96.5% for the heavyweight.
- For regime 2, the fraction of responses that would be spliced, which is
  the whole-trace router's escalation rate minus the prefix catches: on
  the order of 5 to 15% of requests at the current operating points.

## 5. Recommendation

Do not implement regime 2 without a client that has agreed to the
retraction protocol. Regime 1 is implementable in a day, but given the
prefix signal measurements it should be framed as a latency feature of an
always-drafter configuration, not as routing. The honest next step for the
routing story is a whole-trace router on non-streaming traffic, where
buffering is already the contract, and that is where the round 2
measurements point (`docs/RESULTS.md` sections 9 and 11).
