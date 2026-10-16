---
area: deployment
---

Decide deployment details before `deploy-shared-playtest-instance.md`
can be specified:

- Hosting target for the public Go server (what's available/cheap for a
  single small binary + static assets?).
- Base URL configuration: how the server learns its public origin so
  room creation returns working shareable links.
- HTTPS requirement: needed for mobile browsers / touch events /
  any client APIs the frontend relies on?
- In-memory room storage implies a single instance with no failover —
  acceptable for MVP? Anything needed now to avoid rework later
  (e.g., config via env vars rather than hardcoded)?
