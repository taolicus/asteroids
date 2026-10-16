---
area: protocol
dependencies:
  - define-snapshot-format-and-interpolation.md
  - implement-authoritative-game-loop.md
---

# Implement the SSE snapshot stream

GET endpoint streaming game-state snapshots to each connected client,
per the format in `define-snapshot-format-and-interpolation.md`:

- Sequence number and tick on every event.
- Immediate full snapshot on connect (late-join behavior per the
  contract).
- Heartbeats/keepalives so proxies and clients detect dead
  connections.
- Clean teardown on client disconnect.
- Backpressure policy: drop or disconnect slow clients; the sim must
  never block on a client write.

## Acceptance criteria

- `curl -N` on a joined player's endpoint shows ordered snapshots at
  ~10–15 Hz.
- Client disconnects are detected and cleaned up without stopping the
  room.
- A client that stops reading cannot stall the simulation.
