---
area: client
dependencies:
  - define-snapshot-format-and-interpolation.md
  - implement-sse-snapshot-stream.md
  - implement-asteroids-and-weapons.md
---

# Implement the client renderer

Canvas rendering of the shared arena, per the interpolation approach
in `define-snapshot-format-and-interpolation.md`:

- Draw arena, ship(s), asteroids, projectiles; HUD with score and
  survival timer.
- Buffer incoming snapshots and interpolate (~100 ms behind) for
  smooth motion at 10–15 Hz updates.
- Handle sequence gaps, first snapshot, and large jumps (wraparound)
  gracefully.
- Both roles render the same shared view.

## Acceptance criteria

- Two browsers on the same room show consistent, smooth motion with no
  visible stutter from the broadcast rate.
- Renderer copes with a dropped/duplicate event without freezing or
  desyncing badly.

## Outcome

Implemented.

- The frontend is three plain scripts, loaded in order, with no bundler:
  `web/interp.js` (pure interpolation, also `require`-able in Node),
  `web/renderer.js` (canvas drawing), and `web/app.js` (lobby flow,
  stream, render loop). `web/web.go` embeds all three. The behavior is
  documented in `features/renderer.md`.
- `Interp.SnapshotBuffer` implements the settled interpolation: a 100 ms
  render delay, a 12-snapshot buffer, rejection of duplicate/stale `seq`,
  freeze-never-extrapolate at both ends, gap/large-jump snapping (250 ms /
  300 units), shortest-path lerp for positions and headings, entity
  pairing by `id` with the newer snapshot as base, and HUD fields taken
  from the newer snapshot. A fresh buffer per connection resets the `seq`
  watermark, because a reclaimed lobby's sim restarts at 1.
- `Renderer` draws a square arena scaled to the canvas, rocks, shots, and
  the shared ship rotated to its heading (blinking while invulnerable).
  Wraparound entities draw in every copy that can be visible, so a ship
  crossing a seam slides out one side and in the other. The HUD (score,
  survival seconds, lives) is DOM, not canvas.
- `app.js` opens the stream with a streaming `fetch` (not `EventSource`,
  which cannot set the player header or expose the status), parses SSE
  frames, ignores comment heartbeats, and stops polling once the stream
  starts. It reconnects with a doubling backoff (500 ms → 5 s); a
  401/404 re-enters the lobby, a 409 returns to polling. The lobby panel
  stays visible while playing.

Tests: the interpolation cases in `browser/tests/renderer.spec.js` drive
the page's own `Interp` module through `page.evaluate` (100 ms delay,
freeze, wraparound, gap snap, large-jump snap, duplicate rejection, id
pairing and removal). The end-to-end cases play a ready lobby in two
browser contexts and assert the arena and HUD appear, the survival timer
advances on both sides, and the canvas has been drawn. Pure-arithmetic
checks were also run locally under Node. The Go suite is unchanged and
green.

Limitations: the visual "no visible stutter" half of the first acceptance
criterion is not automated — a headless assertion can verify the
mechanics but not the perception. It was left to the two-device
`deploy-shared-playtest-instance.md` playtest. The browser suite cannot
run on the Android dev device, so all Playwright verification is CI-only.
