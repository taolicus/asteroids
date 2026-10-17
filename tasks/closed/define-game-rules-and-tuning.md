---
area: gameplay
---

# Settled game rules and initial tuning

Settled for `implement-asteroids-and-weapons.md` and
`implement-round-flow-and-replay.md`. All distances are sim units; the
client scales the fixed arena to its viewport.

## Arena

- Square, 1000×1000, wraparound on all edges. Square is the best
  shared compromise across portrait phones and landscape desktops.
- The server simulates this fixed arena regardless of device; clients
  only scale rendering.

## Lives and death

- 3 lives, shared by both players.
- Ship–asteroid collision costs one life; ship respawns at center with
  2s invulnerability (rendered flashing). Invulnerability also applies
  at round start.
- 0 lives ends the round.
- A ship is never hit by its own projectiles (no friendly fire).

## Asteroid tiers

- 3 tiers; radius large 40, medium 24, small 14.
- Split: large → 2 medium, medium → 2 small, small is terminal
  (destroyed).
- Speed on split: child speed = parent speed + 30 (fixed increment per
  tier), direction randomized, capped at 200.

## Spawning

- Round start: 5 large asteroids at random positions ≥200 units from
  ship spawn point.
- Then: one new large asteroid enters from a random edge every 7s,
  while total asteroid count < 18.

## Weapons

- Projectiles travel along the turret's aim at 500 units/s, lifespan
  1.5s, maximum 6 live projectiles, fire cooldown 300ms.
- The gunner aims independently of the ship's heading (settled in
  `define-touch-control-scheme.md`): the shot leaves along the posted
  aim, or along the heading when there is none. This changes only the
  fire direction, not these constants.

## Scoring and round end

- Shared score: large 20, medium 50, small 100.
- Survival round: ends only when lives run out. No "clear the field"
  victory; continuous spawning conflicts with it.
- Survival timer counts up from round start to round end. Game over
  shows final score and survival time.
