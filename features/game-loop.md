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

`Snapshot` carries `Seq`, `Tick`, `Round`, `Phase`, `Pilot`, the shared
`Ship`, the round's `Lives`, `Score`, `Survival` (seconds) and `Invuln`
(seconds), plus copies of the asteroid and projectile slices. Clients treat
`Seq` as the ordering/gap key. The slices are copied per snapshot because
snapshots are read on other goroutines; a snapshot never aliases live world
state.

## Rounds and replay

Round flow is `internal/game/round.go` (`roundState`); it is pure and
clock-injected, so the whole phase machine is tested without sleeping.

- A round's phases are `PhaseWaiting`, `PhasePlaying`, and `PhaseOver`.
  The sim opens in `waiting`.
- `waiting` is a fixed `roundStartTicks` (2 s) countdown: the roll is
  announced, but no world exists yet, so `Snapshot.Ship` is the zero ship,
  `Asteroids`/`Projectiles` are empty, `Lives` is `StartLives`, and
  `Score`/`Survival` are zero. The window is a fixed duration, not gated on
  subscriber count: gating would stall a lone player and make tests wait on
  a second client.
- The sim owns one `rng` for its whole life, so rounds differ and a whole
  round sequence is still reproducible from the room's single seed. At each
  round start the sim rolls `Pilot` (`PlayerOne` or `PlayerTwo`) and derives
  that round's world seed from the same `rng`, then creates the world with
  `newWorld(seed, startTick)`. Passing the play-start tick is what makes
  round-start invulnerability and the spawn timer relative to play, not to
  the announcement.
- `playing` steps the world as before. At zero lives the round becomes
  `over`: the world keeps only its existing motion and `Phase` reports it.
- `Sim.Restart` is the replay: `over` starts a new round (announcing fresh
  roles), `waiting` is an idempotent no-op success, and `playing` is
  refused with `ErrRoundInProgress`. Both players share one ship and one
  score, so either may ask for a replay.
- `Sim.RoleOf(player)` and `Sim.Restart()` are blocking loop commands, like
  `SubmitIntent`: the loop stays the only owner of round state, so a caller
  never races the roll.
- The sim is seat-agnostic: it knows `game.Player`, never a lobby seat. The
  server binds owner → `PlayerOne` and guest → `PlayerTwo` in both
  directions (`playerFor`/`seatFor`).

## The world

One round's mutable state — ship, asteroid field, projectiles, lives,
score, and the survival timer — lives in `internal/game/world.go`.

- `newWorld(seed, startTick)` starts a round: the opening field,
  `StartLives` = 3, and round-start invulnerability timed from
  `startTick` (the play-start tick, not the announcement). `Sessions`
  gives each room its own seed, so a round is reproducible from its seed
  while rooms differ.
- `world.step(tick, pilotIn, gunnerIn, dt)` advances it one tick. It is a
  pure function of `(seed, intents, ticks)`: the same seed and the same
  intents replay exactly, which the unit tests assert.
- The step order is fixed and documented: the ship, then the asteroids,
  then spawns, then fire, then projectile motion, then collisions, then
  projectile expiry.
- A `world` is only ever touched by its sim's loop goroutine, so it needs
  no locking, and its slices are copied into each snapshot.

## Asteroids

Rules and tuning are settled in
`tasks/closed/define-game-rules-and-tuning.md`.

- Three tiers (`TierLarge` 40 / 20 points, `TierMedium` 24 / 50,
  `TierSmall` 14 / 100). Large splits into 2 medium, medium into 2 small,
  small is terminal. Each tier's radius and score come from one table, so
  they cannot drift apart.
- The round opens with 5 large asteroids placed at least 200 units from
  the centre, so a spawn is never instantly fatal.
- A new large asteroid enters from a random edge every 7 s, aimed inward
  within ±45° of the edge normal, while the field is below 18.
- Split children inherit the parent's position, get a randomized
  direction, and move at the parent's speed + 30, capped at 200.
- Collisions are measured across the wraparound seam (`wrappedDelta`), so
  objects on opposite edges still hit.

## Weapons and death

- Projectiles are spawned when the gunner holds fire, travel along the
  ship's heading at 500 units/s for 1.5 s, and are capped at 6 live. The
  fire cooldown is 300 ms (9 ticks).
- A projectile consumes itself on the first rock it overlaps, awards that
  tier's score, and splits the rock.
- A ship–asteroid overlap costs one shared life; the ship respawns at the
  centre with 2 s of invulnerability and cannot be hit during it. There
  is no friendly fire: the sim never tests projectiles against the ship.
  At zero lives the round is over.
- When the round is over the world keeps only existing motion (rocks and
  in-flight shots drift), and stops steering, firing, spawning, and
  scoring. The `Ship` is still reported; the stream turns it into `null`
  with `phase` `"over"`.

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
  damping, max speed, and the collision radius used against asteroids).
  They are initial values; there is no settled ship tuning document yet,
  and the collision radius is a starting value until rendering fixes the
  ship's drawn size.
- The loop steps the ship once per tick, before broadcasting, so a
  snapshot is always a whole number of ticks of motion — no half-steps
  or jitter at tick boundaries.

## Intents

Clients send intentions, never state; the contract is settled in
`tasks/closed/define-input-intent-contract.md`.

- `POST /api/lobby/{id}/intent`, authenticated with the same player
  token as the lobby endpoints, which also refreshes presence.
- The payload is role-specific, and the role is the round's roll rather
  than the seat: the server asks the sim (`RoleOf`) which seat is flying
  and which gunning, so a client cannot claim the other role. Decoding is
  strict: an unknown field or the other role's field is a 400.
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
- Both intent handlers share one auth path with the replay endpoint: an
  `authenticate` closure verifies the token, refreshes presence, and
  requires the lobby to be ready (401/404/409) before the body is touched.

## Replay

`POST /api/lobby/{id}/restart` asks the room for a fresh round, using the
same player token and readiness checks as an intent.

- 200 `{"ok":true}` when the round restarts, and also for an already
  waiting round (a second player clicking after the first's replay lands
  is a success, not a conflict).
- 409 while a round is playing or the sim is stopped: a live round cannot
  be reset from under the copilots.
- The round that starts is announced on the same stream the clients are
  already reading, so a replay needs no new endpoint or message: the next
  snapshots carry the new `round` and `pilot`.

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
- A round always ends at game over; nothing claims the round, so a ready
  lobby's sim still starts on its first accepted intent or its first
  stream subscription. Round restart is the explicit replay endpoint
  above, not a sim start/stop.

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
- The DTO is the protocol surface, not `game.Snapshot`. It carries
  `seq`, `tick`, `arena`, `phase`, `round`, `pilot`, `lives`, `score`,
  `time` (the survival timer in seconds), `ship`, `rocks`, and `shots`.
  `phase` is the round's `waiting`/`playing`/`over`; `round` counts the
  room's rounds; `pilot` is the seat flying this round (the other seat
  is the gunner). `ship` is `null` unless the phase is `playing`: there
  is no ship before a round plays or once it is over. Empty
  `rocks`/`shots` are `[]`, never `null`, in every phase.
- Writes carry a deadline, so a client that stops reading is
  disconnected instead of pinning its handler. The sim's capacity-1
  mailbox already means a slow reader can never block the simulation.
