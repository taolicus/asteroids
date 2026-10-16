---
area: protocol
---

Define the snapshot format and client interpolation approach before
`implement-sse-snapshot-stream.md` and `implement-client-renderer.md`
can be specified:

- Snapshot schema: sequence number, tick/timestamp, arena reference,
  ship state (position, heading, velocity, lives), asteroid states,
  projectile states, score, survival timer, round phase.
- Ordering guarantees: can SSE events arrive out of order, duplicated,
  or dropped? What does the client do on a sequence gap?
- Late join: does a joining client receive a full initial snapshot
  immediately, and how does it align with subsequent streamed events?
- Interpolation: recommended buffer (README targets 10–15 Hz broadcast
  over a 30 Hz sim), how far behind to render, snap-vs-smooth behavior
  on large jumps (e.g., wraparound).
