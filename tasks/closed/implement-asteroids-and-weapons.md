---
area: gameplay
dependencies:
  - define-game-rules-and-tuning.md
  - implement-ship-and-arena-simulation.md
---

# Implement asteroids and weapons

Asteroid field and combat, per the rules in
`define-game-rules-and-tuning.md`:

- Asteroid spawn, drift, wraparound; split into smaller tiers when
  shot, with terminal size.
- Gunner fires projectiles (cooldown, speed, lifespan) from intents.
- Projectile–asteroid collision and scoring.
- Ship–asteroid collision and life loss / round-end hook.
- Seeded RNG per room so behavior is testable and reproducible.

## Acceptance criteria

- Unit tests cover: split counts and tiers, scoring increments, ship
  hit handling, projectile cooldown/lifespan.
- Deterministic given a fixed seed.
- Asteroid and projectile state present in snapshots.

## Outcome

Implemented.

- A `world` (`internal/game/world.go`) now owns one round's mutable
  state: the ship, the asteroid field, projectiles, lives, score, and the
  survival timer. `world.step` composes the pure step functions in a
  fixed, documented order (ship, asteroids, spawn, fire, projectile
  motion, collisions, expiry). It is owned by the sim goroutine and is a
  pure function of `(seed, intents, ticks)`, so a fixed seed replays
  exactly.
- Asteroids (`internal/game/asteroid.go`): three tiers in one table with
  their radii and scores (large 40/20, medium 24/50, small 14/100), large
  →2 medium and medium →2 small with small terminal, split children at
  parent speed + 30 capped at 200. The round opens with 5 large rocks at
  least 200 units from the centre; a new large rock enters from a random
  edge, aimed inward, every 7 s while the field is under 18.
- Weapons (`internal/game/projectile.go`): the gunner holds fire;
  projectiles leave along the heading at 500 units/s, live 1.5 s, are
  capped at 6, and respect the 300 ms cooldown. A projectile consumes
  itself on the first rock it overlaps and awards that tier's score.
- Collisions (`internal/game/collision.go`) are measured across the
  wraparound seam, so objects on opposite edges still interact. A
  ship–asteroid overlap costs one shared life and respawns the ship at
  the centre with 2 s of invulnerability, also granted at round start;
  at zero lives the round ends. There is no friendly fire.
- `Snapshot` gained `Lives`, `Score`, `Survival`, `Invuln`, `Asteroids`,
  and `Projectiles`. Slices are copied per snapshot so a snapshot never
  aliases live world state. `NewSim` now takes a seed; `Sessions` gives
  each room its own.
- The SSE DTO in `internal/server/stream.go` maps the new state: `lives`,
  `score`, `time`, populated `rocks`/`shots`, `ship` is `null` and
  `phase` is `"over"` once the lives run out. `ShipTuning` gained the
  collision `Radius` (an initial value until rendering fixes the drawn
  size).

Tests: the tier table and split counts; split-speed step and cap; the
opening field's clearance and count; edge spawns land on an edge and head
inward; drift and projectile motion wrap; `secondsTicks` matches the
settled durations (and catches the 300 ms truncation); seam-aware
distance and overlap; a shot splits a large rock into mediums and scores;
a small rock is destroyed for full score; a ship hit costs a life and
respawns at rest with invulnerability; invulnerability prevents the hit
until it expires; the last life ends the round and freezes play; the fire
cooldown and the live-projectile cap; projectile lifespan; no friendly
fire; the spawn cadence and cap; the survival timer counts up; snapshots
carry the field and never alias it; and two worlds with the same seed stay
identical while different seeds diverge. The stream test now asserts the
opening snapshot's lives, score, ship, and rock list.

Not yet owned here: `phase` `"waiting"`, explicit round start/stop, and
replay — `implement-round-flow-and-replay.md`.
