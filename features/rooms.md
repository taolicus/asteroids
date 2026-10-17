# Rooms

`internal/rooms.Manager` owns two-player room lifecycle;
`internal/server` exposes the HTTP API the frontend uses. Decisions are
recorded in `tasks/closed/resolve-room-lifecycle.md`.

> This describes the implemented create/join model. It is being replaced
> by player-owned lobbies with a permanent link, identity-based seats,
> and ephemeral room instances — see
> `tasks/closed/player-owned-lobby-rooms.md` and
> `implement-player-lobby.md`.

## API

- `POST /api/rooms` → 201 `{id, link}`. The link is relative
  (`/?r=<id>`), so no public base URL configuration is needed yet.
- `POST /api/rooms/{id}/join` body `{token?}` → 200 `{role, token}`;
  404 unknown room; 409 full room.
- `GET /api/rooms/{id}` with `X-Player-Token` header → 200
  `{role, state, pilot, gunner}`; 404 unknown room; 401 missing or
  unknown token.

## Identity and reconnect

- Room IDs and player tokens: 9 random bytes from `crypto/rand`,
  base64url, 12 chars.
- The client stores `{role, token}` in localStorage keyed by room ID.
  Sending that token on join reattaches the role even when the room is
  full, so a refresh or reopened link recovers the seat.

## Presence and expiry

- Authenticated requests refresh last-seen; a player silent for over
  15s reports as disconnected.
- `state` (`waiting`/`ready`) depends on seats filled, never presence:
  a disconnect never unfills a seat or ends a round.
- Expiry runs on one server-wide sweep goroutine (`Manager.Start`,
  1s cadence): rooms empty beyond the grace period or older than the
  hard cap are deleted. Individual rooms own no goroutines.
- Tests inject the clock (`m.now`) and call `Sweep` directly, so
  expiry is tested without sleeping.

## Frontend

`web/app.js`: no `?r=` shows the create-room home; `?r=` auto-joins
(token first, then a fresh seat). The waiting screen shows the role,
share link, and polls status every 2s — a stand-in until
`implement-sse-snapshot-stream.md` provides a live channel. 404 and
409 render readable error states, never a broken screen.
