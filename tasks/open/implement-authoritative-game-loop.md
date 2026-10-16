---
area: server
dependencies:
  - scaffold-project.md
---

# Implement the authoritative game loop

Per-room fixed-timestep simulation:

- ~30 ticks/second on a single goroutine per room (no per-tick
  locking across the room's state).
- One immutable-ish snapshot produced per tick.
- Snapshots broadcast to clients at ~10–15 Hz per the README.
- Room lifecycle: loop starts when a round starts, stops on round end
  or room cleanup; no goroutine leaks.

Simulation content (ship, asteroids) comes in later tasks; this task
delivers the loop skeleton with placeholder state.

## Acceptance criteria

- Tick rate and broadcast rate verified by tests or instrumentation.
- `go test -race` clean; one busy or slow room does not stall others.
- Loop stops cleanly with no leaked goroutines when a room ends.
