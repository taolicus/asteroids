---
area: rooms
---

Open questions about the room/player lifecycle that must be settled
before room management can be implemented:

- Room ID and link format: how are IDs generated, how long can they be,
  what makes a link unguessable?
- A third player opening a valid link: rejected with what message/state?
- Disconnect mid-round: does the round pause, end, or continue with the
  remaining player? Is reconnect possible with the same link and role?
- Empty room (both players gone, round over, nobody joined): expiry
  timing and cleanup.
- One player leaves during the waiting screen: does the room persist for
  a replacement, and for how long?

Resolving these determines the acceptance criteria for
`implement-room-create-join.md`.
