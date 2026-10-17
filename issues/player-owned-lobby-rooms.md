---
area: rooms
---

Room identity should belong to the player, not to an anonymous one-off
room. Proposed tweak to the implemented create/join flow:

- A persistent pseudo-identity per browser (token stored in
  `localStorage`, issued by the server on first visit), independent of
  any room.
- The index page (`/`) *is* the player's own lobby: it immediately
  ensures the player's room exists, shows the player's own share link,
  and offers the browser share API (copy as the fallback). A "join my
  own room" button is an acceptable alternative but the immediate lobby
  is preferred.
- Opening someone else's `/?r=<id>` link seats the player in that room
  as gunner, as today.
- A player is in at most one room at a time, so a player's own room
  needs a `busy` (or similar) state meaning "the owner is playing in
  someone else's room".

## Open questions

- Own-room lifetime conflicts with settled expiry
  (`resolve-room-lifecycle.md`): a 60s empty grace and 30min hard cap
  suit throwaway rooms, but a lobby must survive closing the tab or its
  shared link goes stale. How long does a player's room (and its link)
  live, and what should a stale link do?
- `busy` semantics: what does a visitor see when opening the link of a
  player who is in someone else's room — blocked with an explanation,
  waiting in the lobby, or redirected? Is `busy` a property of the
  player, the room, or both?
- Can a player leave their own lobby to play elsewhere while a guest
  waits in it? Whose seat does the waiting guest hold?
- Identity reset: clearing site data creates a new identity, orphaning
  the old room and any link already shared. Acceptable?
- Does the identity token replace the per-room token for reconnect, or
  sit alongside it?

## Affects

- `tasks/closed/resolve-room-lifecycle.md` — identity, capacity, and
  expiry decisions are partly superseded.
- `tasks/open/implement-room-create-join.md` — index-page and share
  acceptance criteria change; its outstanding two-browser check should
  exercise the new flow instead.
- `features/rooms.md` — API and identity sections change when this
  lands.
