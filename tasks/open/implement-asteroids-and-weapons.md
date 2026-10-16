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
