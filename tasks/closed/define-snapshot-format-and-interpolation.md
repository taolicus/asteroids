---
area: protocol
---

# Settled snapshot format and interpolation

Settled for `implement-sse-snapshot-stream.md` and
`implement-client-renderer.md`; resolves the questions in
`define-snapshot-format-and-interpolation`.

## Transport and authentication

```
GET /api/lobby/{id}/events
X-Player-Token: <owner credential or guest reconnect token>
```

- Server-Sent Events framing (`text/event-stream`), one event per
  broadcast, flushed immediately.
- A comment heartbeat (`: ping`) every 15 s keeps proxies and clients
  from treating an idle stream as dead. Snapshots usually make it
  redundant; it matters when they pause.
- Every write carries a deadline. A client that stops reading is
  disconnected instead of blocking its stream handler, and a slow reader
  never blocks the simulation, because the sim's mailbox is capacity-1
  latest-wins (see Ordering, gaps, and duplicates).
- Authenticated with the same player token as every other player call.
  An open stream counts as presence: the handler refreshes the caller's
  presence at connect and then on a 5 s timer, comfortably inside the
  15 s presence TTL, so a player with a live stream never looks absent.
  When the stream ends, presence lapses after the TTL as usual.
- The lobby must be ready (both players present), like the intent
  endpoint: 409 otherwise, 401/404 as usual. Presence is refreshed
  before that check, so a player reconnecting after their own presence
  lapsed is not rejected by their own absence. Every status is a real
  HTTP status, because the stream has not started yet.
- The client opens the stream when its lobby poll first reports `ready`
  and stops polling then. A 401 or 404 re-enters, exactly as a status
  poll does; a 409 means the room is waiting again, so the client
  returns to polling until it is ready.

### The client uses streaming fetch, not EventSource

`EventSource` is rejected on purpose:

- It cannot set the `X-Player-Token` header, so it would force the
  credential into a query parameter (leaked to access logs) or a
  per-lobby cookie (a second identity mechanism).
- It hides the HTTP status: a 401 (guest seat lost) and a 404 (instance
  reclaimed) both surface as a generic error, but the client already
  distinguishes them to decide how to re-enter.

Streaming `fetch` (`response.body.getReader()`) keeps the existing header
auth and the existing status-aware re-enter logic, and the client
reconnects itself with a small backoff. The framing stays ordinary SSE,
so `curl -N` works and any SSE client could read it.

## Wire schema

One event per broadcast:

```
event: snapshot
id: 412
data: {"seq":412,...}
```

The `data` line is compact JSON on a single line (one `data:` line per
event). The schema:

```json
{
  "seq": 412,
  "tick": 824,
  "arena": 1000,
  "phase": "playing",
  "lives": 3,
  "score": 120,
  "time": 12.4,
  "ship": {"x": 500, "y": 500, "heading": 0, "invuln": 0},
  "rocks": [{"id": 1, "x": 120.5, "y": 800, "radius": 40}],
  "shots": [{"id": 9, "x": 505, "y": 500}]
}
```

- `seq` — per-sim snapshot sequence; the ordering key.
- `tick` — the simulation tick, for diagnostics.
- `arena` — the square arena side. Constant, but sent every event so a
  snapshot is self-describing and the client never hardcodes it.
- `phase` — `"waiting"`, `"playing"`, or `"over"`. Round flow drives it;
  until then a live stream is always `"playing"`.
- `lives`, `score`, `time` — shared HUD state; `time` is the survival
  timer in seconds, counted by the server so both clients agree.
- `ship` — the shared ship, or `null` when there is no live ship (not
  playing). `heading` is radians, 0 along +X; `invuln` is the remaining
  invulnerability in seconds, for the flashing render.
- `rocks` — `id`, position, `radius` (the tier). No velocity: motion
  comes from interpolation, not extrapolation.
- `shots` — `id` and position; radius is a constant the client knows.

`id`s are stable per entity and monotonically increasing per room, so a
client can pair an entity across snapshots. Arrays are always present
(possibly empty), never null.

### The sim's types are not the wire format

`game.Snapshot` is the simulation's own state and will grow fields that
must not be sent (spawn timers, RNG state, collision bookkeeping), and it
holds a velocity the renderer does not need. The SSE task maps
`game.Snapshot` to this schema, a small DTO at the HTTP boundary. The
simulation's exported types carry no wire tags, so nothing implies they
are the contract.

## Ordering, gaps, and duplicates

One HTTP response is one ordered byte stream, so within a connection
events arrive in order, without duplication, and without drops — except
that the server itself drops snapshots for a slow reader. The sim's
mailbox is capacity-1 latest-wins (see `features/game-loop.md`), so a
client that cannot keep up misses intermediate states and then receives
the newest one.

Therefore:

- `seq` is strictly increasing within a connection, and a gap means
  "intermediate states were dropped, and what follows is current". It
  never means corruption, and there is nothing to recover or re-request.
- The client discards any event whose `seq` is not greater than the last
  one it rendered.
- The watermark is reset on every new connection. This matters because
  `seq` is per-sim: a reclaimed lobby gets a fresh sim whose `seq`
  restarts at 1, which would otherwise be below the client's old
  watermark and rejected forever.

There is no replay and no `Last-Event-ID` handling. A reconnect simply
gets the current full snapshot, which is exactly the late-join path, so
no event history needs to be retained. The `id:` field is set to `seq`
for `curl` and debugging.

## Late join

On connect the server subscribes to the room's sim and sends the first
snapshot it receives (at most one broadcast interval, ≈67 ms) as a full
snapshot. It does not synthesize one, so the first event always has a
real `seq` and there is no race between a synthetic snapshot and the
live stream.

Subscribing starts the room's sim if it is not running, exactly as the
first accepted intent does. The lifecycle is still the interim one
recorded in the input-intent contract: a sim with a subscriber is never
reclaimed.

## Interpolation

Broadcast is 15 Hz; rendering is 60 fps. The client renders about 100 ms
in the past, which is longer than one broadcast interval, so it normally
holds the two snapshots that bracket the render time.

- **Timeline.** Each snapshot is stamped with its local arrival time
  (`performance.now()`); no server clock is involved, so there is no
  clock to synchronize. The render time is `now - 100 ms`, and the
  client finds the two snapshots around it and interpolates by `alpha`.
- **Buffer.** Keep the last 12 snapshots (≈800 ms) and drop older ones.
- **Freeze, never extrapolate.** If the render time is newer than the
  newest snapshot, hold that snapshot; do not predict.
- **Wraparound-safe lerp.** Positions interpolate along the shortest
  wrapped path, not the straight line, so a ship crossing an edge slides
  out one side and in the other instead of sweeping across the arena.
  Headings interpolate along the shortest angular delta for the same
  reason. Arena size comes from the snapshot's `arena`.
- **Snap on a jump.** Do not interpolate, just jump to the newer
  snapshot, when the two bracketing snapshots are more than 250 ms apart
  (a hiccup or a long gap), or when a position delta exceeds 300 units
  (a respawn or any real teleport; legitimate per-snapshot motion is at
  most ~32 units for the ship and ~13 for a rock). The delta test uses
  the wrapped distance, and 300 is above every legitimate step.
- **First snapshot.** With nothing to interpolate from, render it as it
  is and start interpolating when the second arrives.

These thresholds are tuning, not contract; the behavior is.

## Out of scope

- Entity spin. Rocks render without rotation; if that is wanted later,
  adding a `rotation` field is backward-compatible.
- Velocity-based extrapolation. If the 100 ms lag ever feels wrong, that
  is the change to consider, and the schema would grow velocities back.
- Per-round role delivery. This stream reports shared game state; which
  controls a player has is the round-flow task's concern. The interim
  mapping is the same as the intent endpoint's: owner is the pilot, guest
  is the gunner.
- Replay from `Last-Event-ID`, and any server-side event history.
