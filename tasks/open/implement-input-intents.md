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
