# Authoritative Game Loop

`internal/game.Sim` runs one fixed-timestep loop per room.

- 30 ticks/second (`TickRate`), single goroutine per sim; room state is
  owned by that goroutine, so no per-tick locking.
- One `Snapshot` value produced per tick; broadcast every 2nd tick
  (`BroadcastEvery`) = 15 Hz.
- Each subscriber gets a capacity-1 "latest wins" mailbox
  (`trySendLatest`): a slow reader drops stale snapshots instead of
  blocking the sim. This is the backpressure policy for SSE clients.
- `Stop` blocks until the loop goroutine exits and closes every
  subscriber channel; `Subscribe` after `Stop` returns an already-closed
  channel. `Stop` on a never-started sim does not hang.

`Snapshot` carries `Seq`, `Tick`, and the shared `Ship`. Asteroid and
round state are added by later simulation tasks. Clients treat `Seq` as
the ordering/gap key.

## Ship and arena

- The arena is a fixed square, `ArenaSize` = 1000 units, wraparound on
  all edges (see `tasks/closed/define-game-rules-and-tuning.md`).
- `Ship` is position, velocity, and heading, with heading in radians and
  0 along +X. It spawns at the arena centre with no motion.
- `stepShip` is the physics: rotation from the pilot's turn axis,
  thrust along the heading, light exponential damping, a speed cap, and
  wraparound. It is a pure function of `(ship, intent, dt)`, so physics
  is tested without real time and behaves identically at any tick rate.
- The ship constants are grouped in `ShipTuning` (turn rate, thrust,
  damping, max speed). They are initial values; there is no settled ship
  tuning document yet.
- The loop steps the ship once per tick, before broadcasting, so a
  snapshot is always a whole number of ticks of motion — no half-steps
  or jitter at tick boundaries.

## Intents

Clients send intentions, never state; the contract is settled in
`tasks/closed/define-input-intent-contract.md`.

- `POST /api/lobby/{id}/intent`, authenticated with the same player
  token as the lobby endpoints, which also refreshes presence.
- The payload is role-specific and the server derives the role from the
  seat (owner = pilot, guest = gunner until per-round role rolling
  lands), so a client cannot claim the other role. Decoding is strict:
  an unknown field or the other role's field is a 400.
- Intents are latest-state. The loop keeps one intent slot per role and
  samples it every tick; a post whose `seq` is not newer is coalesced
  away rather than erroring.
- `SubmitIntent` hands the intent to the loop through the command
  channel and waits for the reply, so the loop remains the only owner of
  simulation state and a caller never observes its own intent
  half-applied.
- An intent goes stale after `intentTTLTicks` (8 ticks, ≈267 ms). The
  loop then applies a neutral intent, so thrust and rotation cannot stay
  stuck when a tab is backgrounded or the network drops. A gap also
  resets the sequence watermark, so a reloaded client starting again at
  `seq` 1 is not ignored.
- A coarse cap of `maxIntentsPerSecond` (2 × tick rate) per role returns
  429 beyond it. It bounds work, not gameplay.

## Per-room sim ownership

`game.Sessions` owns one `Sim` per room.

- `Start` creates and runs a sim on first use and is idempotent; `Get`
  and `Stop` look up and stop one.
- `Sweep` reclaims a sim that has had no intents and no subscribers for
  the idle grace, which callers set to the lobby's `IdleGrace` so the two
  lifecycles agree. A sim with a subscriber — an SSE client — is never
  reclaimed, and its idle clock is kept fresh while it has one.
- Stopping happens outside the registry lock, since `Sim.Stop` blocks
  until the loop exits.

This lifecycle is interim: nothing owns round start yet, so a ready
lobby's sim starts on its first accepted intent.
`implement-round-flow-and-replay.md` will own an explicit start and
stop, which the endpoint and payloads will not notice.

## Snapshot stream

`GET /api/lobby/{id}/events` streams the room's snapshots as
Server-Sent Events. The wire format and the client's interpolation are
settled in `tasks/closed/define-snapshot-format-and-interpolation.md`;
the DTO lives in `internal/server/stream.go`.

- Same player token as the lobby endpoints, which also refreshes
  presence; the lobby must be ready. Every rejection is a real HTTP
  status (401/404/409) because the stream has not started yet. Presence
  is refreshed before the ready check, so a player reconnecting after
  their own presence lapsed is not refused by their own absence.
- An open stream counts as presence and refreshes it on a 5 s timer,
  inside the 15 s `rooms.PresenceTTL`, so a streaming player never looks
  absent.
- Subscribing starts the room's sim if needed; a sim with a subscriber
  is never reclaimed. On disconnect the subscription is cancelled and
  the room keeps running.
- Frames are `event: snapshot`, `id: <seq>`, one compact-JSON `data`
  line. The first event is the first snapshot the subscription delivers,
  so a late join gets full state with a real `seq` and there is no
  synthetic-snapshot race. A `: ping` comment is written every 15 s when
  snapshots pause.
- The DTO is the protocol surface, not `game.Snapshot`. It currently
  carries `seq`, `tick`, `arena`, `phase`, and `ship`, with `rocks` and
  `shots` present but empty; `lives`, `score`, and the survival timer
  arrive with the asteroids and round-flow tasks, and `phase` stays
  `"playing"` until round flow owns the round.
- Writes carry a deadline, so a client that stops reading is
  disconnected instead of pinning its handler. The sim's capacity-1
  mailbox already means a slow reader can never block the simulation.
