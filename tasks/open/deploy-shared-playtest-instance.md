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
