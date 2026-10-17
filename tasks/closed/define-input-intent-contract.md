---
area: protocol
---

# Settled input-intent contract

Settled for `implement-input-intents.md` and
`implement-ship-and-arena-simulation.md`; resolves the questions in
`define-input-intent-contract`.

## Endpoint

```
POST /api/lobby/{id}/intent
X-Player-Token: <owner credential or guest reconnect token>
Content-Type: application/json
```

- Same identity as the lobby endpoints: the owner's signed credential or
  the guest's per-instance reconnect token. The call refreshes the
  caller's presence exactly like a lobby status poll.
- The body is capped at 512 bytes. Intents are intentions only; the
  server never accepts game state.

## Roles

An intent is role-specific, but the payload does not name its role: the
server derives it from the caller's seat, so a client cannot claim the
other role.

Roles are rolled per round (`implement-round-flow-and-replay.md`): the
server finds the seat holding the round's pilot and derives the caller's
role from that, so the payload never names a role and a client cannot
claim the other one. The wire format is the same either way.

## Payloads

Pilot:

```json
{"seq": 7, "rotate": -1, "thrust": true}
{"seq": 8, "steer": -1.5708, "thrust": true}
```

- `seq` — client-side monotonic counter; the first post uses 1.
- `rotate` — integer axis: -1 (counter-clockwise), 0, +1 (clockwise).
  This is the keyboard's control.
- `steer` — optional absolute heading in radians, `0` along +X: the
  direction the touch stick points at. With one the ship turns toward
  that heading and rests on it, so the stick is point-and-go (settled in
  `define-touch-control-scheme.md`); the sim folds the value into
  [-π, π). It wins over `rotate` when both are present. Absent, or
  stale, means `rotate` alone, so the field is additive for a client
  that never steers this way.
- `thrust` — bool.

Gunner:

```json
{"seq": 7, "fire": false, "aim": 1.5708}
```

- `fire` — bool.
- `aim` — optional absolute turret direction in radians, `0` along +X.
  The gunner aims independently of the ship's heading (settled in
  `define-touch-control-scheme.md`); the sim folds the value into
  [-π, π). Omitted, or stale, means the ship's heading, so the field is
  additive for a client that never aims.

Decoding is strict: an unknown field, a wrong type, an empty body, or a
`rotate` outside {-1, 0, +1} is a 400. A gunner sending `rotate` or
`steer` is therefore a 400, not a silent ignore, and so is a pilot
sending `aim` or `fire`.

## Cadence, coalescing, and rate

- Intents are latest-state, not events. The server stores the newest
  intent per role and the tick loop samples it every tick.
- The client re-posts its current intent at 10 Hz even when unchanged,
  and may post immediately when it changes.
- The server drops any intent whose `seq` is not greater than the stored
  one — a post reordered or duplicated by the network — and answers 200.
  Coalescing makes excess harmless, so it is not an error.
- A coarse safety cap of 2 × `TickRate` = 60 intents/second per role
  answers 429 beyond it. It bounds work; it is not a gameplay limit.
- The freshness rule below resets the stored `seq` after any gap longer
  than the freshness window, so a reloaded client that starts again at
  `seq` 1 is not dropped forever.

## Freshness (stale-input auto-stop)

An intent stays live for 8 ticks (≈267 ms at 30 Hz). With no newer
intent, the loop treats that role's intent as neutral: rotation stops
and thrust and fire stop. A backgrounded tab or a dropped network
therefore cannot leave continuous actions stuck on.

This window is deliberately much shorter than the lobby's 15 s presence
TTL: presence answers "is this player connected", freshness answers "is
this held button still held".

## Responses

| Situation | Status |
| --- | --- |
| Accepted, including a coalesced or dropped stale post | 200 |
| Malformed body, unknown field, bad `rotate`, bad `steer`, bad `aim`, or the wrong role's field | 400 |
| No token, or a token holding no seat in this lobby | 401 |
| Invalid lobby ID, or a valid one whose instance has been reclaimed | 404 |
| Lobby not ready (both players not present) | 409 |
| Body over 512 bytes | 413 |
| Over the rate cap | 429 |

Rejections are logged.

The 404 case includes a lobby that exists but has gone idle long enough
to be reclaimed. A client re-enters then, exactly as it already does for
a status poll that returns 404.

## Sim lifecycle (interim)

Nothing owns round start yet, so until
`implement-round-flow-and-replay.md` lands:

- A ready lobby's sim starts lazily on its first accepted intent.
- A sim with no intents and no subscribers for the lobby idle grace
  (60 s) is stopped and reclaimed. A sim with a subscriber — an SSE
  client — is never reclaimed.

Round flow will later own an explicit start and stop. Neither the
endpoint nor the payloads change when it does.
