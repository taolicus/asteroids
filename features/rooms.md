# Lobby rooms

`internal/rooms.Manager` owns ephemeral two-player lobby instances and
their seats; `internal/identity` issues and verifies player credentials;
`internal/server` exposes the HTTP API the frontend uses. Decisions are
recorded in `tasks/closed/player-owned-lobby-rooms.md` and
`tasks/closed/resolve-room-lifecycle.md`.

## Identity

- A player's pseudo-identity is global to the browser: a public, random
  lobby ID (12 base64url chars) plus a secret owner credential, issued
  once by `POST /api/identities` and kept in `localStorage`.
- The credential is `<lobbyId>.<hmac>`, where the HMAC (SHA-256,
  truncated to 128 bits) is keyed by `LOBBY_KEY` and covers the lobby
  ID. It is self-verifying, so the server stores nothing about it and
  identities survive process restarts. `identity.ValidID` rejects
  links that do not have the lobby-ID shape.
- The share link is `/?r=<lobbyId>` and is permanent: closing the tab,
  reclaiming the instance, or restarting the process never invalidates
  it. The credential never appears in the link, so a guest cannot claim
  the owner seat.
- `LOBBY_KEY` must be stable across restarts. If it changes, existing
  credentials stop verifying and those lobbies fall back to guest-only;
  the link still resolves. Without `LOBBY_KEY`, the process generates a
  random key and logs a warning (local development: identities do not
  survive a restart).

## Seats

- Two seats per room: the owner seat, held by whoever presents the
  lobby's owner credential, and the guest seat for everyone else.
  Seating is identity-based, not arrival-ordered: a guest can never take
  someone else's owner seat.
- Roles are *not* seats. Pilot/gunner are rolled 50/50 at each round
  start, and the two are independent: either player flies. The simulation
  keeps only its two players, and the server binds owner → player one and
  guest → player two (`playerFor`/`seatFor` in `internal/server`), which is
  the only place the two vocabularies meet.
- The lobby's `waiting` (below) and a round's `waiting` phase are different
  things: `state` describes the seats, `phase` describes the round about
  to start.
- The guest seat is held while its holder is present (see presence
  below); once the holder has been absent past the presence TTL, the
  seat is free and the next guest takes it with a fresh per-instance
  token.
- A second guest arriving while a guest is present sees the room as
  full (409).

## Presence and state

- Authenticated requests refresh last-seen; a player silent for over 15s
  reports as disconnected.
- `state` is `ready` only while both players are present, `waiting`
  otherwise. A guest waiting in a lobby whose owner is playing
  elsewhere therefore sees `waiting` until the owner returns — the
  owner's seat stays reserved throughout (the "busy" case).

## Instance lifecycle

- Instances are created lazily on first use of a lobby ID. There is no
  durable room storage.
- One server-wide reclaim sweep (`Manager.Start`, 1s cadence) deletes
  instances with no connected player for the idle grace (60s). There is
  no age cap: a lobby a player is waiting in must not be killed. Memory
  is bounded by concurrently connected players.
- Tests inject the clock (`m.now`) and call `Sweep` directly, so
  reclaim is tested without sleeping.

## API

- `POST /api/identities` → 201 `{lobbyId, token}`. Issues a fresh
  identity. No request body or authentication.
- `POST /api/lobby/{id}` with optional `X-Player-Token` → 200
  `{seat, state, owner, guest, path, token?}`. Creates the instance if
  needed and assigns a seat: a valid owner credential for `{id}` takes
  the owner seat, anyone else the guest seat. A newly seated guest
  receives a reconnect `token`. 404 if `{id}` is not a lobby-ID shape;
  409 if the guest seat is taken.
- `GET /api/lobby/{id}` with `X-Player-Token` → the same body without a
  new token, refreshing presence. 404 unknown instance; 401 missing or
  unknown token.
- `path` is the relative share link (`/?r=<id>`), so no public base URL
  is configured.

## Frontend

`web/app.js` is the lobby. `/` shows the player's own lobby (identity
created on first visit) with the permanent link and a share affordance:
the Web Share API when available, otherwise copy-to-clipboard. Both seats
see the same panel — the link and its hint — because nothing in it needs to
say who created the room or who is waiting: the link works for anyone, and
the stream is what says the round is starting. `/?r=<id>` enters that
lobby: the owner credential claims the owner
seat, everyone else joins as guest (a stored per-instance guest token
reconnects to the same seat). There is no create button and no home
screen. The client polls status every 2s while the lobby is waiting for
its second player; once both seats are filled it stops polling, opens
the live snapshot stream, and renders the shared arena
(`features/renderer.md`). It re-enters when an instance has been
reclaimed (404) or a guest seat was lost (401), and returns to polling
if the room is waiting again (409). Full (409) and malformed-link (404)
cases render readable states, never a broken screen. The UI is covered
by browser tests (`features/testing.md`).
