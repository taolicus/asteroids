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

## Outcome

Closed after the playtest: the game was played with other people on the
deployed instance — players on their own devices joined through the share
link and completed rounds, which is the part of the acceptance criteria
the host checks above could not cover.

What those sessions raised was fixed as it came up: the band and stats
leaving the controls stranded in the middle of the screen, the stick's
throw growing with the band (it is held to the band's floor now), and
nothing telling a first-time player where to press (the resting stand-in,
`features/controls.md`). With that, the feel questions this task was named
for — the gunner's relative aim being learnable without instruction,
pull-to-fire firing when the player means it and not while merely aiming,
the pilot's point-and-go reading as a heading, and how far the pull has to
travel — have no answers left outstanding.
