# Verify What The Player Sees

- A style change must change a computed value. Read the current cascade and
  the property's default first: a rule such as `justify-content: flex-start`
  on a flex column, or `align-self: stretch` on a column flex item, is already
  the default. It deploys, passes every check, and changes nothing on screen.
- Layout that only exists in one state — playing, lobby, round panel, error —
  can only be checked in that state. A rule written under `body.playing` is
  invisible from the landing screen.
- When a player reports that a change is not visible, compare what is served
  with what is in the repository before blaming the browser: fetch the file
  with a cache-busting query and hash it against the working tree. The served
  bytes are what the phone has, so a match means the rule was a no-op or is in
  the wrong state, not that the phone is stale.
- Do not keep rules that do nothing. Rules added while diagnosing and then left
  behind read as if they matter, and they hide the one rule that does.
