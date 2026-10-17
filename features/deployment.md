# Deployment

Production runs at `https://asteroids.pifia.net` on the VPS reachable as
`ssh tao`. Deployment is just copying a binary; host setup is already in
place.

## Host setup (already configured)

- systemd unit `asteroids.service`: `Type=simple`, `User=dev`,
  `WorkingDirectory=/home/dev/asteroids`,
  `Environment=ADDR=127.0.0.1:18081`,
  `ExecStart=/home/dev/asteroids/asteroids`, `Restart=always`,
  `RestartSec=3`.
- Drop-in `/etc/systemd/system/asteroids.service.d/lobby-key.conf` adds
  `EnvironmentFile=/etc/asteroids.env`, which holds `LOBBY_KEY` (see
  below). The main unit is left untouched.
- nginx vhost `asteroids.pifia.net`: TLS via Let's Encrypt/certbot,
  reverse proxy to `127.0.0.1:18081`, and an `/events` location with
  `proxy_buffering off` for the upcoming SSE stream. HTTP redirects to
  HTTPS.
- Logs: `ssh tao journalctl -u asteroids`.

## Deploying

`deploy.sh` cross-compiles for linux/amd64
(`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, so it works from the
android/arm64 dev device), copies the binary to the host, replaces it by
rename (never writing a running executable in place), restarts the
service, and polls the public URL until it answers.

```
./deploy.sh
```

The script is POSIX `sh` — `sh deploy.sh` and `bash deploy.sh` both
work. On the dev device the repository sits on a filesystem without the
executable bit, so use `sh deploy.sh` there.

Overridable: `DEPLOY_HOST` (default `tao`), `DEPLOY_DIR` (default
`/home/dev/asteroids`), `HEALTH_URL` (default the public root).

## Listen address

The server binds to `ADDR` (full `host:port`) when set, which is what
the systemd unit provides; otherwise it falls back to `-port`/`PORT`
(default `8080`) for local development.

## LOBBY_KEY

`LOBBY_KEY` signs player credentials (`features/rooms.md`). It must be
stable across restarts and identical for every process serving the same
lobby links, so it lives on the host rather than in the repository. It
is configured on `asteroids.pifia.net`: `/etc/asteroids.env` (root,
`0600`) holds it and the drop-in above makes systemd load it.

To reproduce, create the file and reference it from a drop-in:

```
sudo sh -c 'umask 077; printf "LOBBY_KEY=%s\n" \
  "$(head -c 32 /dev/urandom | base64 | tr -d "=+/")" > /etc/asteroids.env'
sudo mkdir -p /etc/systemd/system/asteroids.service.d
sudo tee /etc/systemd/system/asteroids.service.d/lobby-key.conf <<'EOF'
[Service]
EnvironmentFile=/etc/asteroids.env
EOF
sudo systemctl daemon-reload && sudo systemctl restart asteroids
```

If `LOBBY_KEY` is missing the process starts anyway with a random key
and logs a warning; owner credentials then stop verifying after each
restart (lobbies fall back to guest-only, links still resolve).

## Notes

- A deploy restart drops all in-memory rooms; players rejoin through
  their link and stored token.
- TLS and the proxy config are host concerns, not application concerns
  — the binary only ever speaks plain HTTP to localhost.
