---
area: protocol
dependencies:
  - define-input-intent-contract.md
  - scaffold-project.md
---

# Implement input intent endpoints

HTTP POST endpoint(s) for each player to submit role-specific
intentions, per the contract in `define-input-intent-contract.md`:

- Validate room and player identity; reject unknown/full rooms.
- Reject malformed payloads with 4xx, never crash the server.
- Store the latest intent per player for the tick loop to consume;
  coalesce or drop excess input per the contract.
- Apply stale-input auto-stop per the contract.

Server-authoritative: endpoints accept intentions only, never
game-state mutations.

## Acceptance criteria

- Contract-valid POSTs are accepted and visible to the sim layer.
- Malformed or unauthorized POSTs get 4xx responses and are logged.
- Rate/coalescing and stale-input behavior match the contract.

## Outcome

Implemented to the settled contract
(`tasks/closed/define-input-intent-contract.md`, whose design commit
came first).

- `POST /api/lobby/{id}/intent` authenticates with the same player token
  as the lobby endpoints (which also refreshes presence), derives the
  role from the seat, and rejects with 400/401/404/409/413/429 as the
  contract specifies. Rejections are logged.
- Decoding is strict and per role: the pilot shape is
  `{seq, rotate, thrust}` and the gunner shape is `{seq, fire}`, so an
  unknown field or the other role's field is a 400.
- `game.Sim.SubmitIntent` hands the intent to the loop through the
  command channel and waits for the reply, keeping the loop the only
  owner of simulation state and making the intent visible to the sim
  layer (verified by a working snapshot test).
- Coalescing: the loop keeps one latest intent per role and drops a post
  whose `seq` is not newer instead of erroring. A gap longer than the
  freshness window resets the watermark, so a reloaded client starting
  again at `seq` 1 is not dropped.
- Rate cap: 60 intents/second per role (2 × tick rate) returns 429.
- Stale auto-stop: after 8 ticks (≈267 ms) without a refresh, the loop
  applies a neutral intent, so thrust and rotation cannot stick.

Covered by server tests (statuses, role shapes, coalescing, cap) and sim
tests (visibility, auto-stop, watermark reset, per-role cap).
