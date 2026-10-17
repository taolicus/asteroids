---
area: client
---

Decide the touch control scheme for both roles before
`implement-touch-controls-and-responsive-layout.md` can be specified:

- Pilot: rotate left/right plus thrust — discrete buttons vs a
  virtual joystick; placement for one-handed phone play.
- Gunner: does the gunner aim independently of ship heading (turret
  control) or only fire along the ship's heading? If independent,
  what's the touch input (drag to aim, tap to fire)?
- Dead zones, button sizing, and visual feedback for held buttons.
- Keyboard/mouse fallback for desktop play, without diverging the
  input contract from the touch scheme.

Each player is on their own device (per README success criteria), so
each screen only needs its own role's controls.

The wire contract is already settled
(`tasks/closed/define-input-intent-contract.md`): the pilot posts
`{seq, rotate, thrust}` and the gunner posts `{seq, fire}`. Turret aim,
if adopted, adds an `aim` field to the gunner payload rather than
changing what exists; the client also owns the posting cadence (10 Hz
keepalive) that keeps held buttons alive.
