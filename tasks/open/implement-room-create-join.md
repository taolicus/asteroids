---
area: rooms
dependencies:
  - resolve-room-lifecycle.md
  - scaffold-project.md
---

# Implement room create/join

Two-player rooms with role assignment, per the settled decisions in
`resolve-room-lifecycle.md`:

- `POST /api/rooms` creates a room and returns its shareable link
  (`/?r=<id>`).
- `POST /api/rooms/{id}/join` assigns the first joiner pilot and the
  second gunner, and returns a player token the client stores in
  `localStorage` for reconnect.
- A join carrying a matching token reattaches that role even when the
  room is full.
- Waiting screen until both roles are filled; each client shows its
  own role. The waiting screen polls room status every 2s until
  `implement-sse-snapshot-stream.md` provides a live channel.
- Full and unknown rooms show readable errors (409 / 404), never a
  broken state.
- Expiry follows `resolve-room-lifecycle.md`: 60s empty-room grace,
  30 min hard cap, presence from last-seen activity.

## Acceptance criteria

- Tests cover: create returns a link; joins assign pilot then gunner;
  a third join is rejected as full; an unknown room is rejected as not
  found; a token reattaches a role in a full room; an empty room is
  removed after the grace period; a room is removed at the hard cap.
- Two browsers: one creates a room, the other opens the link; both
  see their assigned role and the waiting screen state.
- Expired rooms release all their state (no unbounded growth).
