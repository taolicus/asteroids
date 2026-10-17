---
area: rooms
dependencies:
  - resolve-room-lifecycle.md
  - scaffold-project.md
  - player-owned-lobby-rooms.md
---

# Implement room create/join

Note: `player-owned-lobby-rooms.md` (settled) supersedes the index-page
and identity parts of this task; see "Closed as superseded" below. The
room API, token reconnect, presence, and sweep logic carry forward into
`implement-player-lobby.md`.

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

## Outcome

Implemented `internal/rooms` (IDs, roles, tokens, presence, expiry with
an injectable clock and one server-wide sweep goroutine),
`internal/server` (the API above), and the room UI in `web/` (create,
join-via-link, role and waiting screen, share link, readable 404/409
states, localStorage tokens). Documented in `features/rooms.md`.

Verified: `go vet` and `go test ./...` pass — rooms and server tests
cover every criteria bullet except the two-browser one — plus an
end-to-end curl smoke test against the running server (create, both
joins, 409/404/401, token reattach, status transitions, share link
page). CI runs the same suite with `-race`.

Outstanding: the two-browser check cannot run on this device (no
browser installed). It stays open until someone opens a created link
in a second browser and confirms both role/waiting states.

## Closed as superseded

`player-owned-lobby-rooms.md` (settled) replaces the index-page and
identity model this task delivered: rooms become player-owned lobbies
with a permanent link, identity-based seats, and no create button. The
outstanding two-browser check is therefore obsolete — it would exercise
a UI that no longer exists — and is covered instead by
`implement-player-lobby.md`'s verification. The room API, token
reconnect, presence, and sweep logic implemented here carry forward and
are extended there.
