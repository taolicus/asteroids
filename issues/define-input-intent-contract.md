---
area: protocol
---

Define the input-intent contract between clients and the server before
`implement-input-intents.md` can be specified:

- Payload schema per role: discrete buttons (rotate left/right, thrust,
  fire) vs analog axes; field names, encoding (JSON?), versioning.
- How intents align with the 30 Hz tick: one POST per event vs a
  continuously updated "latest intent" state per player.
- Rate limits and coalescing: max POST rate, server behavior when
  inputs arrive faster than the tick.
- Stale-input handling: if a client goes silent (tab backgrounded,
  network blip), when does the server auto-stop continuous actions like
  thrust and rotation?
- Validation: what makes an intent invalid, and what is the error
  response?

Server stays authoritative: clients send intentions, never state.
