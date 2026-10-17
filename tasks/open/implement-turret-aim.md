---
area: gameplay
dependencies:
  - define-input-intent-contract.md
  - define-game-rules-and-tuning.md
  - define-touch-control-scheme.md
  - implement-asteroids-and-weapons.md
---

# Implement the independent turret aim

The gunner's turret aims independently of the ship's heading, the decision
`define-touch-control-scheme.md` settles. The touch controls that drive it
belong to `implement-touch-controls-and-responsive-layout.md`; this task is
the protocol, simulation, and rendering half, so it can land and be
verified server-side first.

- The gunner payload gains an optional `aim`: an absolute angle in radians,
  0 along +X, the same convention as `heading`. Absent means the nose — the
  turret points along the ship's heading — so a client that never sends
  `aim` behaves exactly as it does today, and a stale intent falls back to
  the nose rather than freezing the turret off-axis.
- The round's world holds the turret angle and fires along it instead of
  the heading. The turret is presentation and aim state only: it does not
  change the ship's collision radius, the weapon tuning, or the step order.
- The snapshot carries the turret angle — as `turret` on the existing
  `ship` object — so both players can see where the gunner is pointing, not
  just the gunner.
- Strict decoding stands and is not relaxed: the pilot sending `aim`, a
  non-numeric `aim`, or any unknown field is still a 400.
- Amend the two settled documents whose bullets say turret aim is pending:
  the gunner payload in `define-input-intent-contract.md`, and the firing
  direction in `define-game-rules-and-tuning.md`.

## Acceptance criteria

- A gunner intent carrying `aim` fires along that angle, independent of the
  ship's heading.
- A gunner intent without `aim` fires along the heading, exactly as before.
- Both clients receive the turret angle in the stream, and the renderer
  draws it.
- The pilot cannot send `aim` (400), and a malformed `aim` is a 400.

## Verification

Go tests for the payload decode, the independent firing direction, the nose
fallback, and the snapshot field; a browser-suite or local Node check that
the stream carries the angle in the shape the client expects; the existing
weapon tests stay green.
