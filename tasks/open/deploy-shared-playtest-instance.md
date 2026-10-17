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

Host and deployment mechanism are in place and verified
(`features/deployment.md`): the VPS at `asteroids.pifia.net` serves the
deployed build over HTTPS through the systemd unit and nginx vhost;
`deploy.sh` builds the linux/amd64 binary, copies it, restarts the unit,
and health-checks the public URL.

Verified on the host:

- Public HTTPS serves the room UI and API (create/join exercised
  through `https://asteroids.pifia.net`).
- Crash restart: killing the main process (`kill -9`) brought it back
  automatically as a new PID with the service still active.
- Logs: `journalctl -u asteroids` shows startup and the killed process.

Remaining before this can close:

- Run the two-device playtest that the main acceptance criteria require.

The deployed build (most recently the pilot's point-and-go stick,
`tasks/closed/implement-point-and-go-pilot-steering.md`) is live and
serving `controls.js` from the host, so
the playtest can begin. What the playtest has to settle: the round flow and
replay on two devices, the touch feel the controls tasks could not verify
(dead zone, band reach, the fire button, no accidental zoom or page scroll),
the gunner's relative aim — whether "drag the direction you want to point"
is learnable without instruction — and whether the pilot's point-and-go
stick reads as a heading in the hand, including how far the throttle push
has to travel.
