---
area: gameplay
dependencies:
  - define-input-intent-contract.md
  - define-touch-control-scheme.md
  - implement-touch-controls-and-responsive-layout.md
---

# Aim and fire from one gunner stick

The gunner holds two touch controls: a floating aim stick that fills the
band and a separate round fire button in its corner. In the hand that is
two targets for one thumb — the thumb must leave the aim to shoot, so
lining up a shot and taking it are separate reaches — and the button
competes with the stick for the same corner of the band.

The requested behaviour mirrors the pilot's revised stick: *one* joystick
for the gunner, its direction aiming the turret and its pull distance
firing.

## Why the wire does not change

Unlike the pilot's point-and-go change, this needs no protocol work. The
gunner payload already carries `aim` (an absolute direction in radians)
and `fire` (a bool), and the sim already fires while `fire` is held at the
weapon's cooldown rate. A single stick is only a new way to *produce* those
two fields: the drag direction is the same `aim` the aim stick already
sent, and the pull distance sets the same `fire` the button already set.
The button is removed from the client; nothing on the server moves.

## Scope

- The band holds one gunner surface, the aim stick. The fire button is
  removed from the page, the stylesheet, the controls, and the wiring.
- The gunner's stick becomes two-stage, exactly as the pilot's: inside the
  dead zone nothing is sent and the remembered aim stands; past the dead
  zone the drag direction is the aim; past the fire zone (a starting value,
  55% of the stick radius, matching the pilot's throttle zone) it also
  fires. Because the aim is the drag *direction*, pushing further along the
  same ray fires without moving the turret.
- The aim is still remembered after the finger lifts, so the turret rests
  where it was pointed instead of snapping back to the ship's heading; the
  lift only stops the fire, and it stops at once.
- The desktop path keeps working: the mouse over the arena still aims and
  the left mouse button or space still fires while held. A *touch* on the
  arena still never aims.
- Docs: the settled touch scheme is amended, and the controls and testing
  feature notes updated.

## Acceptance criteria

- The gunner has one touch control; the fire button is gone from the page.
- A pull that clears the dead zone but not the fire zone aims without
  firing; a pull past the fire zone does both; the release stops firing at
  once and leaves the aim where it was.
- A pull that follows the first, along the same direction, keeps the aim
  and starts firing without the turret moving.
- The mouse-arena aim and the space/left-button fire still reach the wire,
  and the aim still follows the ship when the pointer is still.
- The pilot's controls and the wire contract are untouched.

## Verification

The browser suite's arithmetic and end-to-end halves for the gunner's stick
and the band's contents; the local Node checks for the shipped helpers
(`controls-check.js`) and the DOM wiring against a fake DOM
(`attach-check.js`); `go test ./...` to show the server is untouched. The
Playwright half runs in CI only, since it cannot run on the dev device.

## Outcome

Delivered in `6fddbf1` ("Aim and fire the gunner's turret from one stick").

As specified: the fire button is gone from the page, the stylesheet, the
controls, and the wiring; the gunner's stick became `stickAim`, the same
two-stage shape as the pilot's `stickSteer` — the drag direction is `aim`,
a pull past `FIRE_ZONE` (55% of the stick radius, its own constant, so it
can be tuned apart from the pilot's throttle) is `fire`, and inside the dead
zone neither is sent, so the remembered aim stands. The mouse and the space
bar still set `fire` directly and merge by OR, so the desktop path is
unchanged. No wire change was needed: the payload already carried `aim` and
`fire`, and the server is untouched beyond a comment.

Verified: the browser arithmetic cases evaluate the shipped `stickAim` (the
dead zone, a light pull aiming without firing, a firm pull firing, and a
longer pull along the same ray leaving the aim alone); the band case asserts
each role sees only their own stick and that `#fire` no longer exists; the
end-to-end case drives one drag from a light pull (an `aim` with no `fire`)
to a firm one (both) to a release (fire off, aim standing). CI is green on
that suite. Locally, `attach-check.js` covered the same path against a fake
DOM plus the space key, `controls-check.js` the helper arithmetic, and
`go test ./...` passes with the server unchanged.

Limitations: whether pull-to-fire fires when the player means it to — and
how far the second stage's pull should travel — is a feel question only a
real device can settle; it stays with
`tasks/open/deploy-shared-playtest-instance.md`, which now names it.
