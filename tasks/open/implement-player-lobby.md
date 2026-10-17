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
