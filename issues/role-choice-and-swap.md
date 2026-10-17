---
area: gameplay
---

Later feature raised by the product owner: controls and mechanisms to
choose and swap roles, so players are not solely at the mercy of the
per-round roll settled in `player-owned-lobby-rooms.md`.

Deliberately out of scope for the MVP; recorded so the round-flow work
does not design itself into a corner.

## Open questions

- Where the controls live: lobby before the round, or also mid-round? A
  mid-round swap hands over a flying ship, so lobby-only is likely, but
  settle it here.
- Mechanism: a per-identity preference stored locally, a swap request the
  other player accepts, or a plain "swap" either player can trigger?
- Interaction with the roll: does a preference override the roll, only
  apply when both agree, and persist across rounds and sessions?
- Fairness: if both players want the same role, who wins?
- Ergonomics that motivate this: device type (phone vs desktop) may make
  one role markedly preferable, and thumb-reach differs between them.

## Affects

- `tasks/closed/implement-round-flow-and-replay.md` — the per-round
  pilot/gunner roll is implemented there; a swap would override it.
- `tasks/closed/player-owned-lobby-rooms.md` — seats vs roles.
- The lobby UI in `web/`.
