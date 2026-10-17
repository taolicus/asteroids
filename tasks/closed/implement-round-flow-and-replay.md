---
area: gameplay
dependencies:
  - define-game-rules-and-tuning.md
  - implement-asteroids-and-weapons.md
  - implement-client-renderer.md
---

# Implement round flow and replay

Shared score, survival timer, game-over, and quick replay:

- Score and timer visible during the round on both clients.
- Round ends per the rules in `define-game-rules-and-tuning.md`.
- Role assignment: at each round start the server rolls pilot/gunner for
  the two players, tells both clients their role before the round begins,
  and keeps it fixed for the round. A player reconnecting mid-round
  resumes that round's role. (Seats and the rationale for the roll are in
  `player-owned-lobby-rooms.md`; choosing or swapping roles is deferred —
  see `role-choice-and-swap.md`.)
- Game-over screen with final score and survival time.
- Quick restart: both players return to a fresh round without
  rejoining the room or reloading.

## Acceptance criteria

- A full round plays from join to game over on two devices.
- Score and timer stay consistent across both clients.
- Roles are rolled per round and both clients see their role before the
  round starts.
- Restart resets the field, score, and timer for both players with no
  reload or rejoin.

## Outcome

Round flow is `internal/game/round.go` (`roundState`), pure and
clock-injected, so the whole phase machine is tested without sleeping.
A sim plays a sequence of rounds: `waiting` → `playing` → `over` →
`waiting`. The opening `waiting` window is a fixed `roundStartTicks`
(2 s) in which the round rolls and announces its roles and has no world
at all — `Snapshot.Ship` is the zero ship and the rock and shot lists are
empty — so both clients are told the roll before anything moves. It is a
fixed countdown rather than a "wait until a player connects" gate: a
gate would stall a lone player, and every test would have to wait on a
second client.

The sim owns one `rng` for its whole life, so rounds differ and a whole
round sequence stays reproducible from the room's single seed. Each round
start rolls `Pilot` (`PlayerOne`/`PlayerTwo`) and derives that round's
world seed from the same `rng`; `newWorld` now takes the play-start tick,
so round-start invulnerability and the first asteroid spawn are timed
from when play begins rather than from the announcement. At zero lives
the round reports `PhaseOver` and the world keeps only its existing
motion, as `implement-asteroids-and-weapons.md` settled.

`Snapshot` gained `Round`, `Phase`, and `Pilot`, and `Sim` gained two
blocking loop commands alongside `SubmitIntent`: `RoleOf(player)` and
`Restart()`. Blocking is what makes them safe — the loop stays the only
owner of round state, so a caller never observes a boundary it raced, and
the intent handler can ask which role a player holds before decoding a
body shaped for it.

The sim remains seat-agnostic (it knows `game.Player`, never a lobby
seat); `internal/server/server.go` binds owner → `PlayerOne` and guest →
`PlayerTwo` in both directions (`playerFor`/`seatFor`), the one place the
two vocabularies meet. The intent and replay endpoints share one
`authenticate` closure (token, presence, readiness → 401/404/409) before
any body is touched. `POST /api/lobby/{id}/restart` is idempotent: a
round that is already waiting answers 200 `{"ok":true}` so a second
player's replay press after the first's is not an error, while a round
still playing is 409 `ErrRoundInProgress`. The DTO gained `round` and
`pilot` (mapped to the seat the client knows), `phase` now comes from the
sim instead of being inferred from lives, and `ship` is `null` unless the
phase is `playing`.

On the client, `app.js` keeps the session's own `seat` and derives the
role from the stream's `pilot` against it — the two roles are always
opposite, so each player reads their own off shared state with no extra
request. The HUD gained a role item, and `#round` shows the two moments a
round pauses: the "Get ready — you're the pilot/gunner" announcement, and
the game-over screen with the round number, final score, and survival
time plus a "Play again" button that POSTs `/restart`. No interpolation
buffer reset is needed at a round boundary: a waiting snapshot has no
ship or rocks, so the next round's first playing snapshot snaps in.

Tests: the whole phase machine on an injected tick (the window closing,
the roll, the HUD before and after, the replay from game over, a replay
while waiting changing nothing, and a replay refused while playing);
deterministic rolls that use both players over 100 rounds; `RoleOf` and
`Restart` plumbing through the sim's command channel; the sim's snapshots
carrying the round, phase, and roll; the server's restart status ladder
(either player may restart, 401/404/409, 200 while waiting) and its
refusal while a round is live; intents accepted only for the seat the
roll gives them (the tests ask the sim rather than assume the owner
pilots); the stream carrying the waiting window with no ship and no
rocks, then the playing field; the DTO mapping for all three phases; and
in the browser, both copilots being told opposite roles before play
begins, with the waiting announcement on screen.

Limitations: the two-device half of the acceptance criteria — a full
round from join to game over, and a replay landing on both players'
screens — is not automated. A real round cannot be driven to game over in
test time (a stationary ship is only hit by chance, and there are no
controls yet to steer into a rock on purpose), so the browser game-over
case serves the page canned snapshots from its route and asserts the
shipped panel code; that is the one deliberate stub in the suite, and it
is documented in `features/testing.md`. The two-device pass is left to
the `deploy-shared-playtest-instance.md` playtest, which is still gated
on touch controls. The browser suite cannot run on the Android dev
device, so all Playwright verification is CI-only.
