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

## Outcome

Implemented to the settled contract
(`tasks/closed/define-snapshot-format-and-interpolation.md`, whose
design commit came first).

- `GET /api/lobby/{id}/events` streams SSE from
  `internal/server/stream.go`, using the same player token as every
  other player call (which also refreshes presence) and requiring a
  ready lobby. Rejections are real HTTP statuses before the stream
  starts: 401 (no/unknown token), 404 (bad ID or reclaimed instance),
  409 (not ready).
- Frames are `event: snapshot` with `id: <seq>` and one compact-JSON
  `data` line carrying `seq`, `tick`, `arena`, `phase`, `ship`, `rocks`,
  and `shots`. The first event is the first snapshot the subscription
  delivers, so a late join gets full state with a real `seq` and there
  is no synthetic-snapshot race. A `: ping` comment heartbeats every
  15 s when snapshots pause.
- An open stream counts as presence and refreshes it on a 5 s timer,
  inside the 15 s presence TTL. Writes carry a 15 s deadline, so a
  client that stops reading is disconnected rather than pinning its
  handler; the sim's capacity-1 mailbox already prevents a slow reader
  from blocking the simulation.
- Subscribing starts the room's sim if needed; on disconnect the
  subscription is cancelled and the room keeps running.
- The DTO is separate from `game.Snapshot`, per the contract. It carries
  the state the sim has today; `lives`, `score`, and the survival timer
  arrive with the asteroids and round-flow tasks, and `phase` stays
  `"playing"` until round flow owns the round.

Verified by `internal/server/stream_test.go` (ordered snapshots and
schema, status codes, disconnect cleanup with the room surviving, the
heartbeat, and a non-reading client not stalling the sim) and a local
`curl -N` smoke showing ~14 Hz ordered snapshots at `tick = seq * 2`.
