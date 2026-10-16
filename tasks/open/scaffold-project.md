---
area: server
---

# Scaffold the project

Greenfield repo; README defines the stack: Go server, vanilla
HTML/CSS/JS frontend, Canvas rendering.

Set up the initial structure:

- `go mod init` with a `cmd/server` entry point.
- Static frontend under `web/` (`index.html`, `style.css`, `app.js`)
  served by the Go server, embedded via `embed` so a single binary
  deploys.
- Configurable listen port; sensible default.
- Zero third-party dependencies (README design principle).

## Acceptance criteria

- `go run ./cmd/server` serves a placeholder page (title + empty
  canvas) in a browser.
- No third-party Go modules; frontend has no build step.
- Run instructions added to the README.
