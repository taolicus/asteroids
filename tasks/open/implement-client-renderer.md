---
area: client
dependencies:
  - define-snapshot-format-and-interpolation.md
  - implement-sse-snapshot-stream.md
---

# Implement the client renderer

Canvas rendering of the shared arena, per the interpolation approach
in `define-snapshot-format-and-interpolation.md`:

- Draw arena, ship(s), asteroids, projectiles; HUD with score and
  survival timer.
- Buffer incoming snapshots and interpolate (~100 ms behind) for
  smooth motion at 10–15 Hz updates.
- Handle sequence gaps, first snapshot, and large jumps (wraparound)
  gracefully.
- Both roles render the same shared view.

## Acceptance criteria

- Two browsers on the same room show consistent, smooth motion with no
  visible stutter from the broadcast rate.
- Renderer copes with a dropped/duplicate event without freezing or
  desyncing badly.
