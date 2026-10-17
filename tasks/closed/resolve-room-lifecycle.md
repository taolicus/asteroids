# Settled room lifecycle

Settled for `implement-room-create-join.md` and later per-room work
(inputs, SSE, rounds). Rendering details stay with the client tasks.

Note: identity, capacity, expiry, and role assignment are revised by
`player-owned-lobby-rooms.md`; where the two disagree, that document
wins.

## Identity and links

- Room ID: 9 bytes from `crypto/rand`, base64url without padding
  (12 chars). Case-sensitive and unguessable.
- Shareable link: `<base-url>/?r=<id>`. The client reads `r` and joins.
- Player token: same 12-char scheme, issued per player on join and
  stored by the client in `localStorage` keyed by room ID.

## Roles and capacity

- Exactly two players, one seat each. Seating is identity-based and
  roles are rolled per round (revised — see
  `player-owned-lobby-rooms.md`). No spectators in the MVP.
- Third joiner: "room is full" (409), shown as a readable state.
- Unknown room ID: "room not found" (404), shown as a readable state.

## Join, reconnect, presence

- `POST /api/rooms` creates a room; `POST /api/rooms/{id}/join`
  returns `{role, token}`.
- A join carrying a token that matches a player of the room reattaches
  that role even when the room is full (refresh or reopen recovery).
  A token that does not match is ignored and the request is treated as
  a new joiner.
- Presence: any authenticated request refreshes that player's
  last-seen time. A player with no request for 15s counts as
  disconnected — a short-polling stand-in until
  `implement-sse-snapshot-stream.md` provides a live channel.

## Disconnect

- A disconnect never ends the round. The sim continues with the
  remaining player's intents: with no pilot there is no rotation or
  thrust; with no gunner there is no fire.
- Reconnecting before expiry resumes their seat and the role of the
  round in progress; the round is not restarted.

## Expiry and cleanup

- A room with no connected players is deleted after a 60s grace
  period, so an accidental refresh or brief network drop does not
  destroy it.
- Hard cap: a room is deleted 30 min after creation regardless of
  activity (rounds last 1–3 min). *Dropped by
  `player-owned-lobby-rooms.md`: idle reclamation is enough, and an age
  cap would kill a lobby someone is waiting in.*
- A waiting room persists while at least one player is connected, so
  the owner can wait for a guest to open the shared link.
- Cleanup runs periodically; an expired room releases all its state
  — including stopping its `Sim` once rounds exist.

## Out of scope

Round start, scoring, end, and replay are settled and implemented by
`implement-round-flow-and-replay.md`. This item only covers room and
player identity up to the point where both roles are filled.
