# Settled deployment and room links

Settled for `deploy-shared-playtest-instance.md`.

## Hosting target

- The existing production VPS (`ssh tao`) already hosts the game at
  `https://asteroids.pifia.net`. No new hosting is needed.
- systemd unit `asteroids.service` runs the binary as user `dev` from
  `/home/dev/asteroids/asteroids` with `Environment=ADDR=127.0.0.1:18081`
  and `Restart=always` (crash restart, `journalctl -u asteroids` for
  logs).
- nginx serves `asteroids.pifia.net`, terminates TLS (Let's Encrypt via
  certbot), and reverse-proxies to `127.0.0.1:18081`, with an
  SSE-friendly `/events` location (`proxy_buffering off`).
- Deployment is a binary copy: `deploy.sh` cross-compiles for
  linux/amd64, copies the binary over SSH, and restarts the unit.

## Base URL configuration

- None needed. Room links are relative (`/?r=<id>`), so they inherit
  whatever origin the player used. The server never needs to know its
  public origin.

## HTTPS

- Provided by the existing nginx/certbot setup. HTTPS is required for
  mobile browser APIs the frontend will rely on (clipboard, later
  touch/vibration), so it is kept rather than bypassed.

## Instance model

- In-memory rooms: a single instance with no failover, accepted for the
  MVP. A restart or deploy drops all live rooms; players rejoin via
  their link and token.
- Runtime configuration stays in the environment (`ADDR` full
  host:port, or `PORT`), so no rebuild is needed to change where it
  listens.
