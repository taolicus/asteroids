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
