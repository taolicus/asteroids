---
area: rooms
dependencies:
  - resolve-room-lifecycle.md
  - scaffold-project.md
---

# Implement room create/join

Implement two-player rooms with role assignment:

- Room creation returns a shareable link containing the room ID.
- First joiner becomes pilot, second becomes gunner (unless
  `resolve-room-lifecycle.md` settles otherwise).
- Waiting screen until both players have joined.
- Full or unknown rooms show a clear error, never a broken state.

Role assignment behavior for a third joiner, disconnects, and room
expiry must follow whatever `resolve-room-lifecycle.md` settles;
update this task's criteria there if unresolved details block work.

## Acceptance criteria

- Two browsers: one creates a room, the other opens the link; both see
  their assigned role and the waiting screen state.
- Opening a link for a full or nonexistent room shows a readable error.
