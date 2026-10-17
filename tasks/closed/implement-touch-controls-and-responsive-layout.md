---
area: client
dependencies:
  - define-touch-control-scheme.md
  - implement-input-intents.md
  - implement-client-renderer.md
  - implement-round-flow-and-replay.md
  - implement-turret-aim.md
---

# Implement touch controls and responsive layout

Mobile-browser support per the README (MVP item 7) and the scheme settled in
`define-touch-control-scheme.md`:

- The pilot's floating virtual joystick: a drag anywhere in its band
  quantizes to the settled `{rotate, thrust}`, with a release posting a
  final neutral intent.
- The gunner's independent aim and fire: a drag in its band aims the turret
  at the touched point, the aim is remembered after the finger lifts, and a
  held fire button shoots.
- A keyboard and mouse fallback for desktop that emits the same intents —
  a second emitter, not a second contract.
- Role-aware control layer: each client shows only the controls of the role
  the round's roll gives it, swapping at a round boundary.
- One 10 Hz poster per client, with the contract's failure handling.
- Responsive layout: fits phone viewports with no scrolling or accidental
  zooming during play, with a control band a thumb can reach.

## Acceptance criteria

- Playable on a phone-sized viewport with touch only, for both roles.
- Touch input produces the expected intents server-side, including the
  gunner's independent `aim`.
- Keyboard and mouse produce the same intents on desktop.
- Layout usable on a recent Android phone browser without pinch-zoom or
  page scroll during play.

## Verification

The browser suite plays a ready lobby for each role and asserts the posted
intents; a local Node or curl check confirms the server decodes them. The
touch feel itself (dead zone, reach, no accidental zoom) is a two-device
playtest item for `deploy-shared-playtest-instance.md`.

## Outcome

Delivered in `bb5528f` ("Give each copilot touch controls and a poster"),
pushed to `main` with CI green (`test` and `browser`; ten new browser cases,
all passing).

Everything is in a new `web/controls.js`, the fourth plain script, written in
the same require()-able shape as `interp.js` so the arithmetic and the poster
can be checked in Node. The page wires it once; which role it serves follows
the stream, so `setRole` is the only thing that changes at a round boundary
and it drops whatever the previous role was holding.

The band is one element per role — the pilot's `#stick`, the gunner's `#aim`
and `#fire` — plus a shared base/knob pair that draws the floating stick. The
band is the touch surface, not the elements inside it, so a touch anywhere in
it is in reach whatever the hand size.

**The gunner's aim is relative, not absolute.** The settled scheme says a
touch aims at the touched point. The band is *below* the arena, so a touch in
it cannot name a point inside the arena and an absolute mapping could only
ever aim downward. The delivered aim is the drag direction from the touch
origin — the same floating-stick gesture as the pilot's — which reaches every
angle, and it is an angle rather than a target. The mouse over the arena keeps
the absolute reading (re-derived every frame, so it follows the ship) and a
touch on the arena never aims. The deviation is recorded in the settled doc
and in `features/controls.md`.

Quantization is the settled 30% dead zone of a 15%-of-the-shorter-side radius.
`fitStick` clamps a long drag to that radius without changing its direction,
so a drag past the ring keeps pointing the same way. The pilot's keys merge
with the stick by OR, so any input can be released while another is held.

The poster (`Controls.createPoster`) starts *paused* and is resumed from the
render loop only while the phase is `playing` and the player is holding
something, so an idle client sends nothing and a control held across the
round boundary is sent when play begins instead of being dropped. A change
posts at once (so a press does not wait for the tick), the 10 Hz tick re-posts
a held intent so it never leaves the contract's 8-tick window, and a *release*
is flagged urgent so it skips the 50 ms floor that otherwise coalesces a fast
drag — without that, the neutral would sit behind the floor and the ship would
coast for a freshness window, which the settled scheme explicitly rejects.
Both failure paths reuse the stream's handling: 401/404 re-enter the room,
409 resumes polling, a network error is retried by the next tick.

Layout: `body.playing` makes `main` one `100dvh` non-scrolling column, hides
the title and the room panel (a phone should spend its height on the arena),
and lets `#stage` take the leftover with the band keeping at least 140 px. The
arena side is measured in `resize()` from the stage box rather than expressed
in CSS: a definite height with `aspect-ratio` and `max-width` over-constrains
one axis and distorts the canvas instead of shrinking it. The stage's flex
basis is 0 for the same reason — the canvas is sized from the box, so a
content-sized basis would be circular.

One thing the delivery fixed that the task did not mention: `web/web.go`'s
`//go:embed` list enumerates files, so the new `controls.js` had to be added
there or the embedded binary would not serve it.

Documents: `features/controls.md` is new (the durable truth, including the
relative-aim rationale); `features/renderer.md` gained the fourth script and
its role; `features/game-loop.md`'s intents section now points at the client
cadence; `features/testing.md` describes the controls spec and drops a stale
"controls do not exist yet" note from the renderer spec description;
`tasks/closed/define-touch-control-scheme.md` carries the amendment above.

Verification, by layer. The browser spec's arithmetic cases drive the page's
own `Controls` module through `page.evaluate` (quantization, the drag clamp,
the drag aim, the seam-crossing mouse aim, the poster's cadence, the urgent
release, and a 401 stopping it). Its end-to-end cases seat two players on the
real server and drive the pads with real pointer input, reading the intent
posts the server receives: each role sees only its own controls, a pilot drag
arrives as `{rotate: 1, thrust: true}` and the release as neutral, a gunner
drag arrives as `aim` in radians with a later fire carrying the remembered
aim, and the keyboard flies the pilot. The layout cases assert a square arena,
the band's height, and no scroll at 390x844 and 844x390. Locally, since
Playwright cannot run on the Android dev device, the same shipped module was
driven through a fake DOM (role gating, quantization, release urgency, the aim
dead zone and the remembered aim, keyboard flight, and mouse aim) and a
one-off end-to-end smoke confirmed the built server accepts the exact bodies
the poster sends and turns the turret to the posted aim.

Limitations: nothing here has been touched by a human thumb. The dead zone,
the band height, the fire-button reach, and "no accidental zoom" are feel
claims that only the two-device playtest can settle, which is
`deploy-shared-playtest-instance.md`. The Playwright half of the suite cannot
run on the Android dev device, so the browser tests were verified in CI only.
