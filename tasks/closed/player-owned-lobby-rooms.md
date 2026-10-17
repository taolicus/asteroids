# Settled player-owned lobby rooms

Settled for `implement-player-lobby.md`. Supersedes the index-page and
identity parts of `implement-room-create-join.md`, and revises the
identity, capacity, expiry, and role sections of
`resolve-room-lifecycle.md`.

## The link is permanent

- Each browser has a persistent pseudo-identity: a public lobby ID plus
  a secret credential, stored in `localStorage`.
- A player's share link is `/?r=<lobby-id>` and is **permanent** — the
  same URL keeps working across tab closes, instance reclamation, and
  server restarts.
- The index page (`/`) *is* that lobby: it shows the player's own link
  with a share affordance (browser share API, copy fallback). There is
  no create button and no separate home screen.

## Room instances are ephemeral

- The room behind a link is an in-memory instance, created lazily the
  first time the link is used and reclaimed after an idle window with no
  connected players (the existing 60s grace; tune later if needed).
- Reclaiming an instance never invalidates a link — the next use simply
  recreates the instance under the same lobby ID. This is the whole
  memory-management story: no durable room storage is needed.
- The 30-minute age cap is dropped: it would kill a lobby that a player
  is legitimately waiting in. Memory is instead bounded by concurrently
  connected players.

## Seats and roles

- Two seats per room: the owner seat, bound to the owner's identity, and
  the guest seat. Seats are what identity protects — a guest can never
  take the owner seat of someone else's room, which is why seating is
  identity-based rather than arrival-ordered ("first joiner is pilot" no
  longer applies).
- Roles are independent of seats: at each round start the server rolls
  pilot/gunner 50/50 for the two players. Every round is a fresh roll, so
  roles may repeat or flip between rounds. The roll is authoritative,
  announced to both clients before the round begins, and fixed for the
  whole round; a player reconnecting mid-round is told that round's roles.
- The identity requirement stands: the credential is self-verifying
  (HMAC-signed by a key from configuration). A client with a valid owner
  identity gets the owner seat; anyone else arriving through the link gets
  the guest seat; a missing or invalid identity is treated as a guest.
- Consequence: the signing key must be stable across restarts (systemd
  environment/file). If it changes, existing owner credentials stop
  verifying and their lobbies fall back to guest-only — the link still
  resolves, but the owner must clear site data to start a fresh identity
  and link.
- Guests need no durable identity: the join-issued per-instance token is
  enough for reconnect within the instance.
- Letting players choose or swap roles is deliberately deferred; see
  `role-choice-and-swap.md`.

## Busy

- A player is in at most one room at a time. If the owner is playing in
  someone else's room, a visitor opening their link **waits in that
  lobby** (taking the guest seat) rather than being rejected; the owner's
  seat stays reserved, and the two meet when the owner returns to their
  own room.

## Guest capacity

- Two players per room, as before: one owner and one guest. A second
  guest arriving while the guest seat is taken sees the room as full, a
  readable state.
