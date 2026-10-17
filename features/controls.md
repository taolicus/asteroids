# Controls

Touch, keyboard, and mouse input for the two copilots, and the intent
poster that sends what they produce. The scheme is settled in
`tasks/closed/define-touch-control-scheme.md` and the wire contract,
including the turret's `aim`, in `tasks/closed/define-input-intent-contract.md`.

All of it lives in `web/controls.js`, the fourth plain script the page loads
(`interp.js`, `renderer.js`, `controls.js`, `app.js`; no bundler). Like
`interp.js` it is require()-able in Node, so the quantization, the aim
geometry, and the poster are checked without a browser.

## The band

One band (`#controls`) sits below the arena. It is in one player's hands at
a time and shows only that role's controls.

- **Pilot — a floating stick.** A touch anywhere in the band puts the stick
  base under the thumb, and the base disappears on release, so one thumb
  reaches it whatever the hand size. The drag quantizes to the settled
  discrete intent: `rotate` is -1 left of the dead zone and +1 right of it,
  `thrust` is on above it. The screen axes and the sim axes agree, so up is
  -y, and a diagonal drag turns and thrusts at once. A drag past the stick
  radius is clamped to the ring, which keeps a long drag pointing the same
  way.
- **Gunner — the same stick, aiming.** The drag direction from the touch
  origin is the aim, in the wire's radians with 0 along +X, so the turret
  reaches every angle. Inside the dead zone the remembered aim stands, so a
  resting thumb cannot spin the gun. The aim survives the finger lifting,
  and firing carries it.
- **Gunner — a fire button.** `#fire` is pinned to the corner of the band,
  within thumb reach, and holds while pressed.

The band follows the round's roll: `app.js` reads the role off the stream
each frame (`pilot` against its own `seat`) and hands it to `setRole()`,
which swaps the visible controls and drops whatever the previous role was
holding. Swapping at the round boundary therefore needs nothing else.

## Why the gunner's aim is relative

The band is *below* the arena, so a touch in it cannot name a point inside
the arena: an absolute mapping would only ever aim downward, which fails the
contract's full 360°. The band is therefore a relative stick — literally the
same gesture and the same floating base as the pilot's — and the aim is the
drag direction.

The desktop path is absolute instead: the mouse over the arena aims from the
ship to the pointer, and `refresh()` re-derives it every frame, so the aim
follows the ship even when the pointer is still. Both paths write the same
`aim` field and the last one wins. A *touch* on the arena never aims: the
band is the gunner's surface. The point-to-ship direction is measured across
the wraparound seam, so aiming at a wrapped copy points the right way.

## Keyboard and mouse

The desktop fallback emits the same intents — a second emitter, not a second
contract:

- Pilot: `ArrowLeft`/`a` and `ArrowRight`/`d` turn, `ArrowUp`/`w` thrusts.
  Held keys are a set, so the stick and the keyboard merge by OR, and a
  window blur clears them.
- Gunner: space or the left mouse button fires; the mouse over the arena
  aims.

## The poster

`Controls.createPoster` sends the intent to `POST /api/lobby/{id}/intent`
with one `seq` counter per poster:

- A change is posted at once, so a press is not held back by the tick, and
  again on every 10 Hz tick, so a held control never goes stale inside the
  contract's 8-tick window.
- A *release* is urgent: it skips the floor that otherwise coalesces a fast
  drag, so the ship stops turning (or stops firing) at once rather than
  coasting for a freshness window.
- The poster runs only while the phase is `playing` and a control is held,
  so an idle client sends nothing. Outside a round it is *paused*, not
  stopped: a control held across the boundary is sent when play begins.
- 401 and 404 stop it and re-enter the room; 409 stops it and resumes
  polling. A network error is transient, and the next tick tries again.

## Layout

While a round is up the page stops scrolling (`body.playing`): the arena
takes the space the HUD and the band leave, the band keeps at least 140 px
of height for a thumb, and the lobby panel is hidden so a phone spends its
height on the arena.

The arena is always square and is sized in `resize()` from the stage box.
It is measured in script rather than expressed in CSS because "the largest
square that fits the leftover space" over-constrains one axis: a definite
height with `aspect-ratio` and a `max-width` clamp distorts the canvas
instead of shrinking it.

Portrait is the intended layout; landscape is supported by the arena
shrinking to the leftover height. The fallback is Chromium on a 390x844
phone viewport and on 844x390, both asserted in the browser suite.
