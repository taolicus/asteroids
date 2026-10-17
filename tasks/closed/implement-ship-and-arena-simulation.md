---
area: gameplay
dependencies:
  - implement-authoritative-game-loop.md
---

# Implement ship and arena simulation

Ship physics inside the authoritative loop:

- Rotation and thrust from the pilot's intents (per the input
  contract).
- Velocity with light damping; position integrated per tick.
- Wraparound at arena edges.
- Ship state present in every snapshot.

Tuning constants should be grouped and easy to adjust; exact values
follow `define-game-rules-and-tuning.md` where it overlaps.

## Acceptance criteria

- Sim tests: thrust accelerates along heading; rotation updates
  heading; ship wraps cleanly across each edge; no jitter at tick
  boundaries.
- Ship state observable in streamed snapshots.

## Outcome

Implemented.

- `Ship` (position, velocity, heading) and `Vec2` join the loop;
  `Snapshot` now carries the ship, so it is observable in every
  broadcast.
- `stepShip` is a pure function of `(ship, intent, dt)`: rotation from
  the pilot's turn axis, thrust along the heading, light exponential
  damping, a speed cap, and wraparound at each edge. Because it is pure
  and takes `dt`, physics is tested without real time and is stable at
  any tick rate.
- The loop steps the ship once per tick before broadcasting, so a
  snapshot is always a whole number of ticks of motion.
- Constants are grouped in `ShipTuning` (turn rate, thrust, damping,
  max speed) as initial values; no settled ship tuning exists yet.

Tests: thrust accelerates along heading and follows a rotated heading;
rotation updates heading both ways and wraps cleanly through ±π; all
four arena edges wrap; velocity decays without thrust; speed is capped;
identical runs are bit-identical and an idle ship never drifts (the
no-jitter criterion); and a thrust intent is visible in the streamed
snapshots.
