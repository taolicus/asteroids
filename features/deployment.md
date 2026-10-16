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

Overridable: `DEPLOY_HOST` (default `tao`), `DEPLOY_DIR` (default
`/home/dev/asteroids`), `HEALTH_URL` (default the public root).

## Listen address

The server binds to `ADDR` (full `host:port`) when set, which is what
the systemd unit provides; otherwise it falls back to `-port`/`PORT`
(default `8080`) for local development.

## Notes

- A deploy restart drops all in-memory rooms; players rejoin through
  their link and stored token.
- TLS and the proxy config are host concerns, not application concerns
  — the binary only ever speaks plain HTTP to localhost.
