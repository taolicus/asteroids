// Pure snapshot interpolation for the renderer.
//
// No DOM and no network: it takes timestamped snapshots and returns a
// view at a render time. The behavior (render ~100 ms behind, keep 12
// snapshots, freeze and never extrapolate, wraparound-safe lerp, snap on
// a jump) is settled in
// tasks/closed/define-snapshot-format-and-interpolation.md.
//
// Loaded as a plain script in the browser; also require()-able in Node,
// so its arithmetic can be checked directly.
(function (root, factory) {
  const api = factory();
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  root.Interp = api;
})(globalThis, function () {
  "use strict";

  const TWO_PI = Math.PI * 2;

  // Tuning, not contract (see the settled doc). delayMs is longer than one
  // broadcast interval, so the render time normally brackets two snapshots.
  const DEFAULTS = {
    delayMs: 100,
    maxSnapshots: 12,
    snapGapMs: 250,
    snapDistance: 300,
  };

  // Shortest signed distance from b to a on an axis of length size.
  function wrappedDelta(a, b, size) {
    let d = (a - b) % size;
    if (d < -size / 2) d += size;
    else if (d >= size / 2) d -= size;
    return d;
  }

  // Interpolate from a to b along the shortest wrapped path.
  function lerpWrapped(a, b, t, size) {
    const d = wrappedDelta(b, a, size);
    return (((a + d * t) % size) + size) % size;
  }

  // Interpolate headings along the shortest angular delta.
  function lerpAngle(a, b, t) {
    let d = (b - a) % TWO_PI;
    if (d > Math.PI) d -= TWO_PI;
    else if (d < -Math.PI) d += TWO_PI;
    return a + d * t;
  }

  // interpShip pairs the shared ship across the two bracketing snapshots.
  // A jump larger than snapDistance (a respawn) snaps to the newer one.
  function interpShip(older, newer, t, size, snapDistance) {
    const base = newer || older;
    if (!base || !base.ship) return null;
    const s = base.ship;
    if (!newer || !older || !older.ship) {
      return { x: s.x, y: s.y, heading: s.heading, invuln: s.invuln };
    }
    const o = older.ship;
    const dx = wrappedDelta(s.x, o.x, size);
    const dy = wrappedDelta(s.y, o.y, size);
    if (Math.hypot(dx, dy) > snapDistance) {
      return { x: s.x, y: s.y, heading: s.heading, invuln: s.invuln };
    }
    return {
      x: lerpWrapped(o.x, s.x, t, size),
      y: lerpWrapped(o.y, s.y, t, size),
      heading: lerpAngle(o.heading, s.heading, t),
      invuln: s.invuln,
    };
  }

  // interpEntities pairs entities by id, taking the newer as the base so a
  // destroyed entity disappears promptly. Unpaired entities do not move.
  function interpEntities(baseList, olderList, t, size, snapDistance) {
    if (!olderList || olderList.length === 0) {
      return baseList.map((e) => ({ ...e }));
    }
    const old = new Map();
    for (const e of olderList) old.set(e.id, e);
    return baseList.map((e) => {
      const o = old.get(e.id);
      if (!o) return { ...e };
      const dx = wrappedDelta(e.x, o.x, size);
      const dy = wrappedDelta(e.y, o.y, size);
      if (Math.hypot(dx, dy) > snapDistance) return { ...e };
      return { ...e, x: lerpWrapped(o.x, e.x, t, size), y: lerpWrapped(o.y, e.y, t, size) };
    });
  }

  // sampleView builds the render view. The HUD fields come from the newer
  // snapshot: they change discretely and should be current.
  function sampleView(older, newer, t, snapDistance) {
    const base = newer || older;
    const size = base.arena;
    const olderPair = older && older !== base ? older : null;
    return {
      seq: base.seq,
      tick: base.tick,
      arena: size,
      phase: base.phase,
      lives: base.lives,
      score: base.score,
      time: base.time,
      ship: interpShip(older, newer, t, size, snapDistance),
      rocks: interpEntities(base.rocks || [], olderPair && olderPair.rocks, t, size, snapDistance),
      shots: interpEntities(base.shots || [], olderPair && olderPair.shots, t, size, snapDistance),
    };
  }

  // SnapshotBuffer holds the recent snapshots on a local arrival clock and
  // samples an interpolated view. It is per connection: a reconnect gets a
  // fresh buffer, which also resets the seq watermark, because a reclaimed
  // lobby's new sim restarts seq at 1.
  class SnapshotBuffer {
    constructor(options) {
      const opts = options || {};
      this.delayMs = opts.delayMs != null ? opts.delayMs : DEFAULTS.delayMs;
      this.maxSnapshots = opts.maxSnapshots != null ? opts.maxSnapshots : DEFAULTS.maxSnapshots;
      this.snapGapMs = opts.snapGapMs != null ? opts.snapGapMs : DEFAULTS.snapGapMs;
      this.snapDistance = opts.snapDistance != null ? opts.snapDistance : DEFAULTS.snapDistance;
      this.snapshots = [];
      this.lastSeq = 0;
    }

    // push accepts a snapshot and its local arrival time. Events that are
    // not newer than the last accepted one are dropped (duplicates, or a
    // stale frame from a previous connection).
    push(snap, at) {
      if (!snap || typeof snap.seq !== "number") return false;
      if (snap.seq <= this.lastSeq) return false;
      this.lastSeq = snap.seq;
      this.snapshots.push({ snap, at });
      const excess = this.snapshots.length - this.maxSnapshots;
      if (excess > 0) this.snapshots.splice(0, excess);
      return true;
    }

    // latest is the most recent accepted snapshot, for diagnostics.
    latest() {
      return this.snapshots.length ? this.snapshots[this.snapshots.length - 1].snap : null;
    }

    // sample returns the view at now, or null before the first snapshot.
    sample(now) {
      const n = this.snapshots.length;
      if (n === 0) return null;
      const renderAt = now - this.delayMs;
      const first = this.snapshots[0];
      const newest = this.snapshots[n - 1];

      // Nothing to interpolate from, or the render time is outside the
      // buffered span: hold a single snapshot, never extrapolate.
      if (renderAt >= newest.at) return sampleView(newest.snap, null, 0, this.snapDistance);
      if (renderAt <= first.at) return sampleView(first.snap, null, 0, this.snapDistance);

      let i = 0;
      for (let k = 1; k < n; k++) {
        if (this.snapshots[k].at <= renderAt) i = k;
        else break;
      }
      const older = this.snapshots[i];
      const newer = this.snapshots[i + 1];
      if (!newer) return sampleView(older.snap, null, 0, this.snapDistance);

      const span = newer.at - older.at;
      // A hiccup or an unusually long gap: jump rather than smear.
      if (span > this.snapGapMs) return sampleView(newer.snap, null, 0, this.snapDistance);
      const t = span > 0 ? (renderAt - older.at) / span : 1;
      return sampleView(older.snap, newer.snap, t, this.snapDistance);
    }
  }

  return { DEFAULTS, wrappedDelta, lerpWrapped, lerpAngle, SnapshotBuffer };
});
