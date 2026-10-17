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

## Outcome

Delivered in `cb5bc50` ("Let the gunner aim independently of the ship"),
pushed to `main` with CI green (`test` and `browser`).

`game.Intent` gained `Aim *float64`. A pointer rather than a value plus a
presence flag, because the field is genuinely optional: `nil` means "no aim
posted" and maps straight from the decoder's optional JSON field, so the
existing `neutral = Intent{}` already falls back to the nose. The world
recomputes `turret` each tick — the ship's heading when there is no aim,
otherwise the posted angle folded into [-π, π) — right after the ship
steps and before it fires, so the nose default uses the current heading and
a shot leaves along the turret. `newProjectile` now takes a position and an
angle instead of the whole ship, which is what makes "fires along the
turret" a property of the call rather than of the ship's heading.

The fallback is deliberately a recomputation, not stored state: an absent
or stale intent returns the turret to the nose instead of freezing it
off-axis, which is the settled behaviour. Folding happens server-side, so a
client accumulating drag past a full turn cannot park the gun at an
unbounded angle; no extra validation is needed because a non-numeric aim
already fails the strict decode.

`Snapshot.Turret` carries the angle out of the sim and the DTO's ship
object gained `"turret"`, so it rides the existing `ship` field that is
already `null` outside a playing phase. On the client `interpShip` lerps
the turret along the shortest angular path but keeps it distinct from the
heading — the hull turning must not drag the gun — and the renderer strokes
a barrel from the ship's centre along the turret angle, falling back to the
heading if a view has no turret. Both players see where the gun points, not
just the gunner.

Besides the acceptance criteria's two settled documents, the snapshot
format record was amended too: it enumerated the ship's wire fields, so it
gained `turret` in both its example frame and its field list.

Tests, by layer: the game tests cover the aim, the nose fallback, the
folding, the spawn direction, and the snapshot field; the server tests
cover strict decoding by seat (gunner `aim` accepted; pilot `aim` and a
text `aim` rejected), the posted aim reaching the running sim, and the DTO
mapping; the browser tests render a synthetic view and check the barrel
lands on the aimed side rather than the heading's, and that the buffer
interpolates the turret independently of the heading. A one-off end-to-end
smoke against the built binary confirmed the whole path, HTTP post to SSE
frame, carries `ship.turret`.

Limitations: the aim is verified server-side and against synthetic views.
The touch input that drives it and the 10 Hz posting cadence that feeds it
belong to `implement-touch-controls-and-responsive-layout.md` and are still
unverified on a real device. As everywhere in this project, the Playwright
half of the suite cannot run on the Android dev device, so the browser
tests were verified in CI only.
