---
area: client
---

# Settled touch control scheme

Settled for `implement-touch-controls-and-responsive-layout.md`; resolves
the questions in `define-touch-control-scheme`. The wire contract stays
`define-input-intent-contract.md`; adopting turret aim added one optional
field to the gunner payload, implemented in
`tasks/closed/implement-turret-aim.md` and recorded in that document
together with the fire direction.

Each player holds one role for a round and is on their own device, so a
screen only ever shows its own role's controls. The client already knows
its role from the stream (`pilot` against its own `seat`), so the control
layer follows the roll and swaps at a round boundary, when the announcement
tells the player what they hold.

## Pilot: a floating virtual joystick

- The band below the arena is the pilot's stick zone. A touch anywhere in
  it puts the stick base down there, so one thumb reaches it whatever the
  hand size; the base disappears on release.
- The drag quantizes to the settled discrete intent. `rotate` is -1 left of
  the dead zone and +1 right of it, else 0; `thrust` is true above the dead
  zone, else false. A diagonal drag turns and thrusts at once, which is the
  reason for a stick over two separate buttons.
- The dead zone is 30% of the stick radius, and the stick radius is 15% of
  the zone's shorter side. Starting values.
- Release posts one final neutral intent instead of relying on the
  freshness window, so the ship stops at once rather than coasting for
  ≈267 ms.

The joystick is an input device, not a new protocol: it emits the existing
`{seq, rotate, thrust}`, which is already discrete.

## Gunner: an independent turret and a fire button

The turret aims independently of the ship's heading.

- Aim: a touch in the aim band points the turret at the touched position —
  the angle from the ship's drawn position to that point, in the wire's
  radians-and-+X convention. Dragging re-aims continuously; the finger
  picks the target and the aim follows the ship as it moves.
- The aim is remembered after the finger lifts. The client keeps posting
  the last aim on its keepalive cadence, so "drag to aim, then hold fire"
  works with one thumb, and a silent client's turret falls back to the
  nose. Nose-forward is also what an absent `aim` means.
- Fire is a held button within the reach of the aiming thumb. Holding it
  fires at the weapon's rate that the settled rules already define. It is a
  separate control rather than firing on the aim touch so the gunner can
  line up a shot without spending it, and so aim and fire can be held on
  two thumbs at once.
- The gunner renders its own aim hint under the touch immediately; the
  turret itself comes from the stream like the ship, one render delay
  behind.

## Keyboard and mouse fallback

Desktop emits the same intents as touch — a second emitter, not a second
contract — so a laptop player and a phone player can share a room.

- Pilot: left/right arrows or A/D rotate; up arrow or W thrusts.
- Gunner: the pointer over the arena aims continuously (the same
  ship-to-pointer angle); the left mouse button or space fires while held.
- The fallback follows the roll too: whichever role a desktop player is
  given for the round is the control set that is live.

## Posting cadence

One poster loop per client, matching the settled contract rather than
inventing a second one.

- Posts the current intent at 10 Hz (the contract's keepalive) and
  immediately on any change, so a fresh press never waits for the tick.
- Keeps one `seq` counter, incrementing per post.
- Posts whichever payload the round's role uses, and switches shape when
  the roll changes.
- Reuses the stream's failure handling: `401`/`404` means the seat is gone
  (stop playing and re-enter), `409` means the room is not ready (stop
  posting and resume polling). At 10 Hz the `429` cap is not reachable.

## Layout

- One column: a compact HUD, the square arena, then the control band. The
  page does not scroll during play.
- The play surfaces (arena and control band) set `touch-action: none`, so
  pan and pinch gestures are absorbed there without disabling page zoom
  globally.
- The arena keeps its 1:1 aspect and takes the space left after the HUD and
  the band; the band has a minimum height of about 140 px so a thumb has
  room. Portrait is the primary case; the same stack fits landscape because
  the arena shrinks.
- The band lays out per role: the pilot's whole band is the stick zone; the
  gunner's band is the aim zone with the fire button placed within thumb
  reach of it.
