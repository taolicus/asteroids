# Testing

Three layers, each covering what the others cannot:

- **Go tests** (`internal/...`) run with `go test ./...`, and with
  `-race` in CI. They are the fast, primary layer: identity signing,
  seat assignment, presence, and reclaim are all tested here. Time is
  injected (`Manager.now`) so expiry is tested without sleeping.
- **Browser tests** (`browser/`) drive the served frontend in a real
  browser. They are the only layer that exercises the DOM, the
  `localStorage` identity, the two-player seating flow, and the canvas
  renderer end to end.
- **A manual curl smoke test** is used when checking a deployment
  (identity → owner → guest → ready, plus the 4xx states).

## Browser tests

Playwright, under `browser/`. The suite starts the real server itself
(`webServer`: `go run ./cmd/server` from the repository root, with a
test `LOBBY_KEY` and `ADDR=127.0.0.1:18099`), so it exercises the same
embedded frontend and API that production serves — no mocks.

A "second browser" is a second Playwright browser context: contexts do
not share `localStorage`, so they hold independent identities and
seats, which is exactly the manual two-browser check.

`browser/tests/fixtures.js` holds the shared `players` fixture and
`shareLink` helper; spec files import them, and the file is not itself
collected as a test.

The renderer spec (`browser/tests/renderer.spec.js`) has three halves. The
interpolation cases run `page.evaluate` against the page's own `Interp`
module, so they test the shipped arithmetic (the 100 ms delay, the freeze,
wraparound, gap/large-jump snapping, duplicate rejection, and id pairing)
rather than a copy. The end-to-end cases play a ready lobby in two
contexts and assert the arena and HUD appear, the survival timer advances,
the canvas has actually been drawn, and each copilot is told their rolled
role for the round. Note that a headless browser can run the interpolation
cases without a second player; only the end-to-end cases need the guest.
The round screens end with a game-over case that serves the owner page
canned snapshots (see below).

The controls spec (`browser/tests/controls.spec.js`) has three halves. The
arithmetic cases run `page.evaluate` against the page's own `Controls`
module: the quantization (dead zone, both axes, the diagonal), the drag
clamp, the aim from a drag, the absolute aim across the wraparound seam,
and the poster (a change posted at once, a release skipping the coalescing
floor, the 10 Hz tick, and a 401 stopping it). The end-to-end cases seat two
players, drive the pads with real pointer input, and read the intent posts
the server receives: the two roles see only their own controls, a pilot drag
arrives as turn-and-thrust and the release as neutral, a gunner drag arrives
as `aim` in radians and a later fire carries the remembered aim, and the
keyboard flies the pilot. The layout cases set a phone and a landscape
viewport and assert the arena is square, the band has its height, and the
page does not scroll.

Run locally:

```
cd browser
npm ci
npx playwright install --with-deps chromium
npx playwright test
```

CI runs it as a separate `browser` job (`.github/workflows/ci.yml`), so
a browser failure never blocks or slows the Go job. On failure the HTML
report and traces are uploaded as an artifact.

### What stays out of the browser suite

Idle-grace reclaim is deliberately Go-only. A browser test would have
to wait out the real 60s grace, whereas the Go test advances an injected
clock through it deterministically. Do not add a reclaim test here.

The round phase machine is likewise Go-only. `internal/game/round_test.go`
walks `waiting → playing → over → waiting` with an injected tick, so it
asserts the phases, the roll, and the replay rules without any sleeping;
a browser test could only wait on the same 2s announcement window, less
deterministically.

One browser test does stub the stream, and it is the deliberate exception
worth knowing about: the game-over panel cannot be reached from a real
round in test time (a round runs until three lives are lost, which takes
minutes even when a player steers into rocks on purpose), so the round
screens case `page.route`s `/events` and feeds the owner page canned
game-over snapshots. Everything downstream of the stream — the panel, the
stats, the replay POST — is the shipped code; only the data is canned. Do
not use this pattern to test anything the real server can produce.

### Device constraint

The browser suite cannot run on the Android dev device: Playwright
refuses to load on the `android` platform, there is no Chromium, and
the shared-storage checkout does not support the symlinks `npm install`
needs. Author and run it in CI or on the laptop.
