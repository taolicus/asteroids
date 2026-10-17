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
