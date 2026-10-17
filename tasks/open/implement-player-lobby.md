---
area: rooms
dependencies:
  - player-owned-lobby-rooms.md
  - implement-room-create-join.md
---

# Implement player-owned lobby rooms

Turn rooms into player-owned lobbies per the settled decisions in
`player-owned-lobby-rooms.md`:

- Persistent player identity: a public lobby ID plus a self-verifying
  (HMAC-signed) credential, issued on first visit and stored in
  `localStorage`. The signing key comes from configuration
  (`LOBBY_KEY`), stable across restarts; document it and set it on the
  host.
- Room instances become ephemeral: created lazily under a lobby ID and
  reclaimed after the idle window. Drop the age cap. Extend the sweep
  and its tests accordingly.
- Seats follow identity: the owner holds the owner seat and a guest the
  guest seat, regardless of arrival order; an unknown or invalid identity
  is treated as a guest. Roles are *not* tied to seats — they are rolled
  at each round start (`implement-round-flow-and-replay.md`).
- The index page is the player's own lobby: no create button, shows the
  room, the permanent link, and a share affordance using the browser
  share API with a copy fallback.
- Busy: an owner who is in someone else's room leaves a guest waiting in
  their own lobby; the guest sees that they are waiting for the owner.

## Acceptance criteria

- Tests cover: identity signing and verification (valid, tampered, wrong
  key); owner and guest get their respective seats in either arrival
  order; an invalid identity is treated as a guest; a reclaimed room is
  recreated under the same lobby ID on the next use; a second guest sees
  the room as full.
- The same link keeps working after the instance is reclaimed and after a
  process restart with the same key, and the link string is unchanged.
- The index page shows a lobby with a share affordance and no create
  button; opening someone else's link joins as guest.
- Manual two-browser check on a device with browsers (this one has none).

## Outcome

Implemented:

- `internal/identity`: HMAC-signed owner credentials (`<lobbyId>.<sig>`)
  keyed by `LOBBY_KEY`, self-verifying and stateless; `ValidID` guards
  the link shape.
- `internal/rooms`: lobby instances keyed by permanent lobby ID,
  created lazily, reclaimed after the idle grace (age cap removed),
  identity-based owner/guest seats, presence-based guest seat with
  per-instance reconnect tokens, `waiting`/`ready` from presence.
- `internal/server`: `POST /api/identities`, `POST /api/lobby/{id}`
  (enter/re-enter), `GET /api/lobby/{id}` (poll); 404/401/409 states.
- `web/`: `/` is the player's own lobby with a permanent link and a
  share affordance (Web Share API, copy fallback); `/?r=<id>` joins a
  lobby as guest; no create button or home screen. Polls every 2s and
  re-enters after reclaim (404) or a lost guest seat (401).
- `cmd/server`: reads `LOBBY_KEY`, warns and uses a random key when it
  is unset. `features/rooms.md` and `features/deployment.md` document
  the model and the host key setup.

Verified: `gofmt`, `go vet`, and `go test ./...` pass (identity signing
and tampering, wrong key, seat assignment in either arrival order,
invalid identity as guest, reclaim-and-recreate under the same ID, no
age cap, second guest full, credential surviving a restart with the
same key and failing with a different key), plus an end-to-end curl
smoke test against the running binary (identity → owner → guest →
ready, 409/401/404, owner-absent guest waiting, index served). CI runs
the suite with `-race`.

Deployed to `https://asteroids.pifia.net`:

- `LOBBY_KEY` set on the host via a systemd drop-in and
  `/etc/asteroids.env` (`features/deployment.md`); verified loaded into
  the running process.
- `deploy.sh` shipped the lobby build; the public index serves the
  lobby markup with no create button, and the live API exercises
  identity → owner → guest → ready, 409 full, and 404 malformed.
- Restart check: after `systemctl restart asteroids`, a credential
  issued before the restart still claims the owner seat, while a bad
  signature falls back to guest — the link and credential are stable
  across restarts.

Outstanding:

- The two-browser check cannot run on this device (no browser). It
  stays open until the lobby is opened in two browsers and the owner/
  guest states are confirmed visually. This is the only criterion left.

