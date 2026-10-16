# Continuous Integration

`.github/workflows/ci.yml` runs `go vet ./...` and `go test -race ./...`
on every push and pull request, on GitHub-hosted `ubuntu-latest`.

- The repo `taolicus/asteroids` is public, so Actions minutes are free
  and unlimited. No secrets are configured; the job uses the automatic
  `GITHUB_TOKEN` with `contents: read`.
- `setup-go` installs the exact toolchain from the `go` directive in
  `go.mod`.
- Runners have gcc, so `-race` (which requires cgo) works there. This
  covers devices where `-race` is unsupported, e.g. Termux/android.
  Local verification on non-linux/amd64 devices should treat CI as the
  authoritative `-race` run.
- Inspect runs with `gh run list` / `gh run watch <id>`.
