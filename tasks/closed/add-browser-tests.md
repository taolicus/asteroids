---
area: testing
---

# Add browser tests for the lobby UI

Automate the frontend check that this device cannot perform (no browser
on the phone), by driving the served app in a real browser from CI.

- Add a Playwright project under `browser/`, pointed at the running
  binary (Playwright starts the server itself via `webServer`).
- Use separate browser contexts as the "two browsers" of the manual
  check: isolated `localStorage` means isolated identities and seats.
- Keep it a separate CI job from the Go tests, so a browser failure
  never blocks or slows the fast Go job.
- The idle-grace/reclaim behaviour stays Go-only: it is already covered
  with an injected clock, and a browser test would need real waits past
  the 60s grace. Do not add a reclaim test to the browser suite.

## Acceptance criteria

- Tests cover: the index page is the player's own lobby with a
  permanent link and no create button; the identity survives a reload;
  a second context opening the share link joins as guest and both see
  the lobby ready; a third context sees it as full; a malformed link
  shows a readable error.
- The suite runs in its own CI job on every push, starting the server
  from the built binary.
- `go test ./...` is unchanged and still does not need a browser.

## Outcome

Added a Playwright project under `browser/` (`@playwright/test` 1.64.1,
pinned with a lockfile; no dependency added to the Go module). The
config starts the real server itself (`webServer`: `go run ./cmd/server`
from the repo root, test `LOBBY_KEY`, port 18099), and the six tests
cover the index lobby with a permanent link and no create button,
identity persistence across reload, a second context joining as guest
with both sides reaching ready, a third context seeing full, a
malformed link error, and clipboard copy.

CI runs it as a separate `browser` job (its own job, so it never blocks
the Go job), installing Chromium and uploading the report on failure.
Documented in `features/testing.md`, which also records that idle-grace
reclaim stays Go-only.

The suite runs on every push and both jobs are green.

Two things worth noting:

- Playwright cannot run on the Android dev device at all (it reports
  `Unsupported platform: android`, there is no Chromium, and the
  shared-storage checkout cannot create the symlinks `npm install`
  needs), so the suite is CI-only from here.
- On its first run the suite immediately found a real bug: author
  `display` rules (`.share`, `section`) overrode the user-agent
  `[hidden] { display: none }`, so hidden views — the guest's share
  block, and the lobby/error toggle — were still rendered. Fixed with
  `[hidden] { display: none !important; }` in `web/style.css` and
  deployed; the tests now assert the lobby/error exclusivity.

