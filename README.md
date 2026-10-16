# Two Idiots, One Ship

## Overview
A browser-based, two-player cooperative micro-game inspired by the classic arcade game Asteroids. Players share one spaceship and must survive an asteroid field together. The game is designed for quick, accessible sessions lasting 1–3 minutes, joined through a simple shareable link.

## Gameplay
- **Pilot:** Controls ship rotation and thrust.
- **Gunner:** Controls the ship's weapons.
- Both players share the same ship, lives, and score.
- Asteroids drift through a wraparound arena and split into smaller pieces when shot.
- The objective is to destroy as many asteroids as possible and survive as long as possible.

## Architecture
- **Backend:** Go server responsible for authoritative game simulation, collision detection, scoring, player sessions, and room management.
- **Frontend:** HTML, CSS, JavaScript, and HTML Canvas for rendering and touch-friendly controls.
- **Communication:** Clients submit player intentions through HTTP POST endpoints. The server streams game-state updates to clients using Server-Sent Events (SSE).
- **Simulation:** Fixed timestep of approximately 30 ticks per second, broadcasting state snapshots at approximately 10–15 Hz. Clients interpolate snapshots for smooth rendering.
- **Deployment:** A publicly hosted Go server serves the game and manages multiplayer sessions. Players join using a shareable room link, with no installation required.

## Running
Requires Go (see `go.mod` for the version). No frontend build step and no
third-party dependencies.

```
go run ./cmd/server
```

The server listens on port `8080` by default. Override it with the `-port`
flag or the `PORT` environment variable:

```
PORT=9000 go run ./cmd/server
go run ./cmd/server -port 9000
```

`ADDR` (a full `host:port`) takes precedence over both; the production
systemd unit uses it. See `features/deployment.md`.

Then open `http://localhost:8080/` in a browser.

## MVP Scope
1. Create and join a two-player room through a link.
2. Assign players to the pilot and gunner roles.
3. Render a shared asteroid arena on both clients.
4. Implement movement, thrust, shooting, asteroid physics, collisions, and splitting.
5. Synchronize game state through SSE and accept inputs through HTTP POST.
6. Add a shared score, survival timer, game-over screen, and quick replay.
7. Support mobile browsers with responsive layouts and touch controls.

## Design Principles
- Start playing within seconds of opening a link.
- Keep rounds short, immediately understandable, and replayable.
- Use simple arcade visuals and minimal UI.
- Keep the server authoritative; clients send intentions rather than trusted game-state changes.
- Keep the implementation small and avoid unnecessary infrastructure or dependencies.

## Initial Success Criteria
Two players on separate devices can join the same room, control their assigned roles simultaneously, see consistent game state, and complete a full round without significant input lag or synchronization issues.