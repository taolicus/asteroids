#!/bin/sh
# Build the server binary and deploy it to the production VPS.
#
# Deployment is deliberately just "copy the binary": systemd
# (asteroids.service) and nginx (asteroids.pifia.net) are already set
# up on the host. See features/deployment.md.
#
# POSIX sh only: works with sh, dash, bash, or ash.
set -eu

HOST="${DEPLOY_HOST:-tao}"
REMOTE_DIR="${DEPLOY_DIR:-/home/dev/asteroids}"
SERVICE="asteroids"
HEALTH_URL="${HEALTH_URL:-https://asteroids.pifia.net/}"

repo_root="$(cd "$(dirname "$0")" && pwd)"
cd "$repo_root"

build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

echo "==> building for linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$build_dir/asteroids" ./cmd/server

echo "==> copying to $HOST:$REMOTE_DIR"
scp -q "$build_dir/asteroids" "$HOST:$REMOTE_DIR/asteroids.new"
# Replace by rename so a running binary is never written in place.
ssh "$HOST" "chmod 0755 '$REMOTE_DIR/asteroids.new' && mv -f '$REMOTE_DIR/asteroids.new' '$REMOTE_DIR/asteroids'"

echo "==> restarting $SERVICE"
ssh "$HOST" "systemctl restart $SERVICE"

echo "==> checking $HEALTH_URL"
i=0
while [ "$i" -lt 20 ]; do
  if curl -fsS -o /dev/null "$HEALTH_URL"; then
    echo "==> deployed"
    exit 0
  fi
  i=$((i + 1))
  sleep 1
done

echo "deploy failed: $HEALTH_URL did not come up" >&2
exit 1
