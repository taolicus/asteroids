---
area: client
dependencies:
  - define-touch-control-scheme.md
  - implement-input-intents.md
  - implement-client-renderer.md
---

# Implement touch controls and responsive layout

Mobile-browser support per the README (MVP item 7) and the scheme in
`define-touch-control-scheme.md`:

- Role-specific touch controls for pilot and gunner.
- Controls emit input intents through the existing POST contract;
  no separate mobile input path.
- Responsive layout: fits phone viewports with no scrolling or
  accidental zooming; comfortable one-handed reach.
- Desktop keyboard/mouse fallback if the scheme calls for it.

## Acceptance criteria

- Playable on a phone-sized viewport with touch only, for both roles.
- Touch input produces the expected intents server-side.
- Layout usable on a recent Android phone browser without pinch-zoom
  or page scroll during play.
