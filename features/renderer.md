# Client Renderer

The browser draws the shared arena from the SSE snapshot stream. The
frontend is three plain scripts, loaded in order (`web/index.html`); there
is no bundler or build step. `web/web.go` embeds all of them.

- `web/interp.js` — pure interpolation: no DOM, no network. Exposes
  `Interp` (and `module.exports`, so Node can read it) with
  `wrappedDelta`, `lerpWrapped`, `lerpAngle`, and `SnapshotBuffer`.
- `web/renderer.js` — canvas drawing only. Exposes `Renderer.create`.
- `web/app.js` — the lobby flow, the snapshot stream, and the render loop.

## Interpolation

`Interp.SnapshotBuffer` implements the behavior settled in
`tasks/closed/define-snapshot-format-and-interpolation.md`:

- Each accepted snapshot is stamped with its local arrival time
  (`performance.now()`). `sample(now)` renders at `now - delayMs` (100 ms
  by default), which normally brackets two snapshots; it interpolates
  between them with `t = (renderAt - older.at) / (newer.at - older.at)`.
- The buffer keeps the last 12 snapshots and drops anything not newer than
  the last accepted `seq` (duplicates and stale frames).
- It never extrapolates: before the first snapshot it holds the first,
  and past the newest it freezes on the newest.
- It snaps (uses the newer snapshot directly) instead of smearing when the
  two bracketing snapshots are more than 250 ms apart, or when a paired
  entity moved more than 300 units.
- Positions interpolate along the shortest wrapped path, and headings
  along the shortest angular path.
- Entities are paired by `id`, with the newer snapshot as the base, so a
  destroyed rock or expired shot disappears promptly and a new one appears
  at its spawn position.
- The HUD fields (`lives`, `score`, `time`) come from the newer snapshot
  regardless of `t`: they change discretely and should be current.

The reminder that `seq` restarts at 1 on a reclaimed lobby is why the
buffer is per connection: `app.js` builds a fresh `SnapshotBuffer` every
time the stream connects, which resets the `seq` watermark.

## Drawing

`Renderer.create(canvas).render(view, nowMs)` draws one frame:

- The arena is square (`game.ArenaSize` = 1000); the scale is
  `canvas.width / arena`. The canvas is sized to its CSS box times
  `devicePixelRatio` on load and on every resize.
- Wraparound entities are drawn in every copy that can be visible
  (`eachWrapped` checks the nine wrapped offsets), so a ship crossing a
  seam slides out one side and in the other.
- Rocks are stroked circles; shots are small filled circles; the shared
  ship is a filled triangle rotated to its heading, blinking to a lower
  alpha while `invuln > 0`.
- The HUD is DOM (`#hud`), not canvas, so it stays crisp and testable:
  score, survival seconds, and lives.

The lobby panel stays visible while playing; the arena and HUD are
unhidden when the round starts and hidden again on exit.

## Stream and reconnection

`app.js` opens the stream with a streaming `fetch` (not `EventSource`,
which cannot set the `X-Player-Token` header or expose the status). It
reads the body, splits SSE frames on `\n\n`, ignores comment heartbeats,
and JSON-parses the `data:` payload. The lobby poll stops once the stream
starts.

On a dropped stream it reconnects with a doubling backoff (500 ms → 5 s).
A `401`/`404` means the seat is gone: it stops playing and re-enters the
lobby (which recreates or reclaims the seat). A `409` means the room is
waiting for a second player again: it stops playing and resumes polling.
