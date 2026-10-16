---
area: deployment
dependencies:
  - determine-deployment-and-room-links.md
  - implement-touch-controls-and-responsive-layout.md
---

# Deploy a shared playtest instance

Put the game on a publicly reachable server so real two-device
playtests are possible, per `determine-deployment-and-room-links.md`:

- Deploy the single Go binary (embedded static assets) to the chosen
  host.
- Configure the public base URL so created room links work for any
  recipient.
- HTTPS if the deployment decision requires it.
- Basic ops: restart on crash, a way to fetch logs.

## Acceptance criteria

- Two players on separate devices, different networks, join via a
  shareable link and complete a full round — meeting the README's
  initial success criteria.
- Room links open and work for a recipient who was not involved in
  creating the room.
- The instance comes back up on its own after a crash, verified by
  stopping the running process and confirming it restarts.
- Server logs can be retrieved from the host to diagnose a failed
  playtest.

## Progress

Host and deployment mechanism are in place (`features/deployment.md`):
the VPS at `asteroids.pifia.net` has the systemd unit, nginx TLS vhost,
and SSE-ready `/events` proxy; `deploy.sh` builds the linux/amd64 binary,
copies it, restarts the unit, and health-checks the public URL.

Remaining before this can close:

- Deploy the current build and verify the crash-restart and log
  criteria on the host.
- After gameplay lands (this task also waits on touch controls), run
  the two-device playtest that the main acceptance criteria require.
