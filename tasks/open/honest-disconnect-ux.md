# Honest disconnect and absence UX

Requested after a disconnection review: enumerate what players see when
someone leaves, then fix the worst of it. The inventory and prioritisation
live in the conversation that produced this task; the decisions below are
what was chosen.

## Current gaps being fixed

- The SSE stream never carried presence, and polling (the only thing that
  reads lobby state) stops while streaming, so a player whose friend left
  was never told: `Play again` 409'd into a silent catch (a dead button),
  and touching a control silently dumped the player back to the lobby
  panel mid-round.
- The error screen's only exit was Reload, which loops on "This lobby is
  already full." while the URL still carries `?r=`.
- Absence always cost the full 15 s presence TTL, even for a deliberate
  tab close.

## Decisions

- **Mid-round absence keeps the round playing.** The remaining player
  finishes their run solo; the absent player's intents already go neutral
  within ~267 ms (sim intent TTL), so nothing gets stuck. A small
  "Friend disconnected" badge over the arena is the only mid-round change.
- **At game over the round panel is honest.** While the other seat is
  absent, `Play again` is replaced by "Waiting for your friend to come
  back…"; it reappears the moment presence returns.
- **The stream carries presence.** The server emits an SSE `presence`
  event when the other seat's presence flips, and treats the open stream
  as presence exactly as before. An intent 409 mid-round marks absence on
  the client instead of leaving the round screen.
- **The poster revives instead of dying.** `Controls.createPoster` stops
  on 409; a new `revive()` clears that when the room is ready again, so a
  player who keeps holding through a disconnection resumes sending without
  re-pressing.
- **A deliberate close says goodbye.** `pagehide` fires a
  `navigator.sendBeacon` to `POST /api/lobby/{id}/leave`, which marks the
  seat absent at once (the token rides in the body because sendBeacon
  cannot set headers). Backgrounding or locking a phone does not beacon:
  the stream/TTL still cover it and the player keeps their seat.
- **The error screen can go home.** A "Go to my room" button navigates to
  `/`, ending the "already full" reload loop.
- **Declined (P4):** distinguishing "waiting to join" from "the host is
  not here yet" in the lobby copy. The API already returns per-seat
  presence, but the shared panel was deliberately left owner-blind (see
  `tasks/closed/player-owned-lobby-rooms.md`); revisit only with a
  stronger reason than clarity.

## Verification

- Go: rooms `Leave`, the leave endpoint, and stream presence events (with
  a fake clock via a `rooms.Manager` clock seam).
- Browser (CI): the disconnect badge mid-round with the round still
  playable, and the error screen's way home.
