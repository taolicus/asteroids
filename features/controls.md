# Controls

Touch, keyboard, and mouse input for the two copilots, and the intent
poster that sends what they produce. The scheme is settled in
`tasks/closed/define-touch-control-scheme.md` and the wire contract,
including the turret's `aim` and the pilot's `steer`, in
`tasks/closed/define-input-intent-contract.md`.

All of it lives in `web/controls.js`, the fourth plain script the page loads
(`interp.js`, `renderer.js`, `controls.js`, `app.js`; no bundler). Like
`interp.js` it is require()-able in Node, so the stick geometry, the aim
geometry, and the poster are checked without a browser.

## The band

One band (`#controls`) sits below the arena. It is in one player's hands at
a time and shows only that role's controls.

- **Pilot — a floating stick, point-and-go.** A touch anywhere in the band
  puts the stick base under the thumb, and the base disappears on release, so
  one thumb reaches it whatever the hand size. The drag *direction* is the
  heading the nose should point at, sent as the contract's optional `steer`
  (an absolute angle, 0 along +X), so a push in any direction turns the ship
  and holding a direction holds a course. The drag *distance* is the
  throttle: inside the dead zone nothing is sent, a light push steers
  without burning, and a push past the throttle zone also thrusts. `steer`
  is a heading rather than an axis because the ship must end up *pointing*
  where the stick does, and the ship is turned server-side (below). A drag
  past the stick radius is clamped to the ring, which keeps a long drag
  pointing the same way.
- **Gunner — the same stick, aiming and firing.** One stick, as for the
  pilot: the drag direction from the touch origin is the aim, in the wire's
  radians with 0 along +X, so the turret reaches every angle, and the drag
  *distance* is the trigger. A light pull aims without spending a shot, a
  pull past the fire zone also fires, and inside the dead zone neither is
  sent, so the remembered aim stands and a resting thumb cannot spin the
  gun. The aim survives the finger lifting; the lift only stops the fire.

Both sticks share a radius of 35% of the band's shorter side — a thumb's
throw, and enough range for the distance to carry the second stage. The
dead zone is 30% of that radius for both, and the pilot's throttle zone and
the gunner's fire zone are 55%, kept as separate constants so either can be
tuned alone. All four are starting values. The radius is a fraction, not a
pixel count, so the throw is the same in portrait and landscape; the band's
own minimum height is what keeps it reachable on a phone.

The band follows the round's roll: `app.js` reads the role off the stream
each frame (`pilot` against its own `seat`) and hands it to `setRole()`,
which swaps the visible controls and drops whatever the previous role was
holding. Swapping at the round boundary therefore needs nothing else.

## Why the ship is turned server-side

`steer` names a heading, and the sim turns the ship toward it and stops on
it: `turnDelta` clamps the tick's rotation to the remaining error, so the
ship lands exactly on the stick direction instead of overshooting it and
hunting.

The obvious alternative — the client comparing the stick against the heading
it has and posting `rotate` — was rejected. The streamed heading is 150–250 ms
old (a 15 Hz stream plus the renderer's 100 ms interpolation delay), and a
bang-bang turn-toward loop against that lag overshoots by roughly the turn
rate times the lag, about 1 rad at 150 ms. Stabilising it needs either a
deadband wide enough to make the nose sloppy or client-side prediction with
a measured round trip and a duplicated turn rate. The server has the stick
direction and the heading in the same tick, so it turns exactly, with no
loop to tune.

The keyboard keeps the discrete `rotate`, and the server prefers `steer`
when a payload carries one, so the field is additive: a client that never
steers this way behaves as it always did.

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

Because the aim is the drag *direction* and not the drag *distance*, one
stick can carry both: pulling further along a ray fires without moving the
turret. That is what makes a single stick enough, and why the fire button
was dropped — a second target for the same thumb costs the aim to reach it.

## Keyboard and mouse

The desktop fallback emits the same intents — a second emitter, not a second
contract:

- Pilot: `ArrowLeft`/`a` and `ArrowRight`/`d` turn, `ArrowUp`/`w` thrusts.
  Held keys are a set, so thrust merges with the stick by OR, and the stick's
  `steer` wins over the keys' `rotate` while it is out of its dead zone. A
  window blur clears them.
- Gunner: space or the left mouse button fires; the mouse over the arena
  aims. That fire merges with the stick's trigger by OR, so a desktop
  player fires without the stick and a touch player never needs the button.

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
