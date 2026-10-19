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
- A closing tab does not wait out the grace: `pagehide` sends a beacon to
  `POST /api/lobby/{id}/leave` (see below), which marks that seat absent
  at once. Backgrounded or locked devices do not fire `pagehide`, so
  those still rely on the TTL — which is the point of having both.

## Instance lifecycle

- Instances are created lazily on first use of a lobby ID. There is no
  durable room storage.
- One server-wide reclaim sweep (`Manager.Start`, 1s cadence) deletes
  instances with no connected player for the idle grace (60s). There is
  no age cap: a lobby a player is waiting in must not be killed. Memory
  is bounded by concurrently connected players.
- Tests inject the clock (`m.now`, or `Manager.SetClock` from outside the
  package) and call `Sweep` directly, so reclaim is tested without
  sleeping.

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
- `POST /api/lobby/{id}/leave` with `{"token": "<player token>"}` in the
  body → 204, marking that seat absent at once. The token rides in the
  body because `navigator.sendBeacon` cannot set headers; it authenticates
  exactly like everywhere else. The owner's credential and a live guest
  token both work, an unknown one is 401, and leaving never creates the
  instance the way entering does (404). A stale guest token (the seat has
  since been taken by someone else) is refused too, so a late beacon
  cannot mark the current holder absent.
- `path` is the relative share link (`/?r=<id>`), so no public base URL is
  configured. It describes the room that was entered; the client builds its
  share box from its own identity instead, so a guest is never shown the
  owner's link as their own.

## Frontend

`web/app.js` is the lobby. `/` shows the player's own lobby (identity
created on first visit) with the permanent link and a share affordance: the
Web Share API when available, otherwise copy-to-clipboard. The share box is
built from the player's own identity and never from the room the page was
opened into: a player always carries their own permanent link, whatever
seat they hold elsewhere.

`/?r=<id>` enters that lobby: the owner credential claims the owner seat,
everyone else joins as guest (a stored per-instance guest token reconnects
to the same seat). There is no create button and no home screen. The owner
of a waiting room sees the link and a "Waiting for friend" line; a guest in
an away room sees the away notice instead — the room is someone else's and
its owner is not here — with no share box at all, because the room is not
theirs to share. Each waiting panel carries a small spinning indicator that
runs only while it is on screen.

Waiting cannot end without the other player, so leaving is one behaviour:
the away notice, the round panel, and the error screen all navigate to `/`,
which drops any other player's `?r=` from the address bar and leaves the
seat they held to the `pagehide` beacon.

The client polls status every 2s while the lobby is waiting for its second
player; once both seats are filled it stops polling, opens the live snapshot
stream, and renders the shared arena (`features/renderer.md`). It re-enters
when an instance has been reclaimed (404) or a guest seat was lost (401),
and returns to polling when the stream itself refuses to open because the
room is waiting again (409). Full (409) and malformed-link (404) cases render
readable states, never a broken screen — and the error screen offers "Go to
my room" (`/`), because reloading would keep the bad link in the URL and land
on the same error. The UI is covered by browser tests
(`features/testing.md`).

### Disconnects and absence

The stream is the only channel that can report the other seat while a
round runs, so it carries it: the server writes `event: presence` with
`data: {"present": <bool>}` whenever the other seat's presence flips
(both start present, since a ready lobby is what opens a stream). The
client reads it as `game.opponentPresent` and reacts on the next frame:

- Mid-round, a "Friend disconnected" badge appears over the arena and
  nothing else changes: the round plays on solo, because a restart would
  throw away the surviving player's round. The absent player's intent
  goes neutral within the sim's own freshness window, so the ship does
  not fly on a stale held input.
- At game over, `Play again` is replaced by "Waiting for your friend to
  come back…" — a restart needs both seats, so the button would only
  409 — and comes back the moment presence does. While it waits, "Go to my
  room" (`/`) sits beside it, so a player left on that panel is not stuck
  with a reload as their only exit.
- An intent 409 (the room is not ready) means a seat is absent, so it
  marks the absence instead of dropping the player back to the lobby
  panel mid-round. The poster stops on that 409 and `revive()`s when the
  friend returns, so a player who kept holding a control resumes sending
  without re-pressing.

A friend who returns within the TTL is invisible to all of this: their
own request refreshes presence and the round resumes mid-flight.
