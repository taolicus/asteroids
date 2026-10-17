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
