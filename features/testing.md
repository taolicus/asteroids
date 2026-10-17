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

The renderer spec (`browser/tests/renderer.spec.js`) has two halves. The
interpolation cases run `page.evaluate` against the page's own `Interp`
module, so they test the shipped arithmetic (the 100 ms delay, the freeze,
wraparound, gap/large-jump snapping, duplicate rejection, and id pairing)
rather than a copy. The end-to-end cases play a ready lobby in two
contexts and assert the arena and HUD appear, the survival timer advances,
and the canvas has actually been drawn. Note that a headless browser can
run the interpolation cases without a second player; only the end-to-end
cases need the guest.

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

### Device constraint

The browser suite cannot run on the Android dev device: Playwright
refuses to load on the `android` platform, there is no Chromium, and
the shared-storage checkout does not support the symlinks `npm install`
needs. Author and run it in CI or on the laptop.
