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
- Role assignment: at each round start the server rolls pilot/gunner for
  the two players, tells both clients their role before the round begins,
  and keeps it fixed for the round. A player reconnecting mid-round
  resumes that round's role. (Seats and the rationale for the roll are in
  `player-owned-lobby-rooms.md`; choosing or swapping roles is deferred —
  see `role-choice-and-swap.md`.)
- Game-over screen with final score and survival time.
- Quick restart: both players return to a fresh round without
  rejoining the room or reloading.

## Acceptance criteria

- A full round plays from join to game over on two devices.
- Score and timer stay consistent across both clients.
- Roles are rolled per round and both clients see their role before the
  round starts.
- Restart resets the field, score, and timer for both players with no
  reload or rejoin.
