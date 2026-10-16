---
area: gameplay
---

Settle core game rules and initial tuning constants before
`implement-asteroids-and-weapons.md` and
`implement-round-flow-and-replay.md` can be specified:

- Arena size and whether it scales with device/viewport or is fixed.
- Ship death conditions: does colliding with an asteroid cost a life or
  end the round? How many lives? Friendly fire (ship vs own projectiles)?
- Asteroid split tiers: sizes, how many pieces per split, terminal size,
  speed change on split.
- Initial spawn count, spawn cadence (do new asteroids keep entering?),
  spawn placement rules.
- Scoring: points per asteroid destroyed, by tier; shared score between
  both roles as per README.
- Round end: all lives lost, or survive until the field is cleared?
  Survival timer semantics (counts up until death?).
