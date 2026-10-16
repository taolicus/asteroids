---
area: gameplay
dependencies:
  - define-game-rules-and-tuning.md
  - implement-asteroids-and-weapons.md
  - implement-client-renderer.md
---

# Implement round flow and replay

Shared score, survival timer, game-over, and quick replay:

- Score and timer visible during the round on both clients.
- Round ends per the rules in `define-game-rules-and-tuning.md`.
- Game-over screen with final score and survival time.
- Quick restart: both players return to a fresh round without
  rejoining the room or reloading.

## Acceptance criteria

- A full round plays from join to game over on two devices.
- Score and timer stay consistent across both clients.
- Restart resets the field, score, and timer for both players with no
  reload or rejoin.
