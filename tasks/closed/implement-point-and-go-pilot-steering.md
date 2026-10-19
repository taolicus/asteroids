---
area: gameplay
dependencies:
  - define-input-intent-contract.md
  - define-touch-control-scheme.md
  - implement-touch-controls-and-responsive-layout.md
---

# Point the pilot's stick and steer the ship there

The pilot's touch stick is awkward in the hand: the settled scheme
quantizes the drag per axis, so thrust only comes from a pull *up* and
rotation only from a pull to the sides, and the stick radius is so small
(15% of the band's shorter side, ~21 px on a phone) that the pull
distance carries no information at all.

The requested behaviour is point-and-go: the stick points where the ship
should be facing, so the stick's *direction* turns the ship however it is
pushed, and the pull's *distance* is the throttle.

## Why this is a protocol change

Turning the ship "toward where the stick points" is a heading command,
which the current pilot payload cannot express: it carries only the
discrete `rotate` axis. The client could close the loop itself against
the streamed heading, but the heading arrives 150–250 ms late (15 Hz
stream + the renderer's 100 ms), and a bang-bang turn-toward controller
with that lag overshoots by roughly the turn rate times the lag — about
1 rad at 150 ms — so it hunts unless the deadband is wide enough to make
the nose sloppy, and tightening it needs client-side prediction plus a
measured round trip. The server has the stick direction and the ship's
heading at the same instant, so it can turn exactly, with no loop, no
prediction, and no duplicated tuning. That is why this adds a field
instead of a client heuristic; the payload addition mirrors the turret's
optional `aim`, which is the same "absolute direction" idea.

## Scope

- Pilot payload gains an optional `steer`: an absolute heading in radians,
  0 along +X, the same convention as `aim`/`heading`. With one, the ship
  turns toward that direction at the turn rate and stops *on* it rather
  than overshooting; without one, the discrete `rotate` axis turns the
  ship exactly as today, so the keyboard and every existing client keep
  working. `steer` wins when both are present.
- `game.Intent.Steer *float64`, folded into [-π, π) server-side like `aim`.
- The touch stick becomes point-and-go: the drag direction is the steer,
  the drag distance a two-stage throttle (a light push steers, a firm
  push also burns), and the dead zone clears both. The stick radius grows
  so the distance has a usable range.
- The gunner's aim, the fire button, and the keyboard keep their current
  behaviour.
- Docs: the settled intent contract and touch scheme are amended, and the
  feature notes for controls and the game loop updated.

## Acceptance criteria

- A pilot post with `steer` makes the ship's heading converge to that
  angle and rest on it, from any starting heading and across the wrap
  seam, without overshooting it.
- A pilot post with only `rotate` turns the ship exactly as before.
- The keyboard path is unchanged and still drives the ship.
- The stick's distance controls thrust: inside the dead zone nothing is
  sent, a light push steers only, a firm push also thrusts.
- The gunner cannot send `steer`, and a malformed `steer` is a 400.

## Verification

Go tests for the steering maths (convergence, no overshoot, shortest
path, the fold, the discrete-axis fallback) and for the payload decode
and the whole post-to-stream path; the browser suite for the stick
arithmetic and the end-to-end post; the local Node checks for the client
helpers and the wire bodies; `go test ./...` locally. The Playwright half
still runs in CI only, since it cannot run on the dev device.

## Outcome

Delivered in `7c1cbe4` ("Point the pilot's stick and steer the ship there").

As specified: the pilot payload gained the optional `steer`
(`game.Intent.Steer *float64`, decoded per seat), `stepShip` turned it into
`turnDelta`, which turns toward the angle at the turn rate and clamps the
step to the remaining error so the nose rests on it, and the touch stick
became `stickSteer` — direction as the steer, distance as a two-stage
throttle — with the radius raised from 15% to 35% of the band's shorter
side and a new 55% throttle zone.

Verified: new `internal/game` tests for turning toward the direction,
resting without overshoot or chatter, the shortest way across the seam, the
fold, the axis fallback and precedence, and a new intent replacing the
steer; `internal/server` tests for the decode (a text `steer` and a
gunner's `steer` are 400s) and for the whole post-to-stream path
(`TestPilotSteerReachesTheSim` posts -2.5 rad and reads the heading back off
the stream). The browser suite was rewritten for the new arithmetic and the
end-to-end post (a light push arrives as a heading without thrust, a firm
one also thrusts, the release is neutral), and CI is green on that suite.
Locally, `wire-smoke.mjs` drove the built binary and watched the heading
settle at -2.5000 while the gunner's `aim` kept working, and the fake-DOM
`attach-check.js` covered the drag, the light push, and the release.

Limitations: the playtest feel — how far the throttle push should travel,
and whether a heading command reads as "point and go" in the hand — is not
something the dev device can settle; it was left to
`tasks/closed/deploy-shared-playtest-instance.md`, which named it.

