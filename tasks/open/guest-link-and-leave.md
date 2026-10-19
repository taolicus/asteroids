---
area: protocol
---

# A guest's link, and a way out of someone else's room

Opening someone else's `/?r=<id>` link fills the share box with *that*
link: `render()` writes the room payload's `path` (`/?r=` + the room's ID)
straight into `#share-link`, and the Share button shares it too, so a
visitor is invited to hand out a room they do not own. If the room's owner
is away the page then sits on "Waiting for friend" with a spinner and no
way out but editing the URL or reloading — the error screen, whose "Go to my
room" button exists for exactly this, is never reached.

Confirmed against the deployment before changing anything: two fresh
identities, host `/?r=zRKC8rVCV1yh`, visitor `/?r=eO6VmC_7Nro_`; the
visitor entering the host's link came back `seat=guest path=/?r=zRKC8rVCV1yh`,
which is what the visitor's box showed.

## Decisions

- **The share box is always the player's own room.** It is built from the
  identity's own lobby ID, never from the `path` the API returns for the
  room that was entered. For an owner the two are the same link, so nothing
  changes; for a guest the box stops being somebody else's.
- **A guest in an away room gets a notice, not the panel.** When the seat is
  guest and the room is waiting, the page says the room is someone else's
  and its owner is not here, with its own spinner and "Go to my room". No
  share box: the room is not theirs to share. (Chosen over showing the
  guest's own link in the shared panel.)
- **Leaving is one behaviour.** `#guest-leave`, the round panel's
  `#round-leave`, and the error screen's `#home` all navigate to `/`, which
  clears the other player's `?r=` from the address bar; the existing
  `pagehide` beacon releases the held seat at once.
- **Game over offers the same way out.** While the friend is gone, the round
  panel shows the waiting line *and* "Go to my room", so a player is not
  left with a reload as the only exit. Both swap back when presence returns.
- The API keeps returning `path`; the client simply no longer trusts it for
  its own box. `session.path` was dead and is gone.

## Verification

- Browser (CI): a guest opening an away room is told so and has no share
  box, and leaving lands on their own room with their own link, no `?r=`;
  the game-over panel shows the way out only while the friend is gone, and
  pressing it navigates.
- Local (no Playwright on this device): the fake-DOM harness drives the
  shipped `render`, `show`, `updateRoundPanel`, and the leave handlers.
