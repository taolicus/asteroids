# Authoritative Game Loop

`internal/game.Sim` runs one fixed-timestep loop per room.

- 30 ticks/second (`TickRate`), single goroutine per sim; room state is
  owned by that goroutine, so no per-tick locking.
- One `Snapshot` value produced per tick; broadcast every 2nd tick
  (`BroadcastEvery`) = 15 Hz.
- Each subscriber gets a capacity-1 "latest wins" mailbox
  (`trySendLatest`): a slow reader drops stale snapshots instead of
  blocking the sim. This is the backpressure policy for SSE clients.
- `Stop` blocks until the loop goroutine exits and closes every
  subscriber channel; `Subscribe` after `Stop` returns an already-closed
  channel. `Stop` on a never-started sim does not hang.

`Snapshot` currently carries only `Seq` and `Tick`; ship, asteroid, and
round state are added by later simulation tasks. Clients treat `Seq` as
the ordering/gap key.
