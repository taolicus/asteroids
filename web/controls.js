// Input for the two copilots, plus the poster that sends what it produces.
//
// The scheme is settled in tasks/closed/define-touch-control-scheme.md:
// one floating stick per role, plus the same intents from a keyboard and
// mouse. Both sticks are point-and-go with a second stage — the drag
// direction is the heading the nose should point at (the pilot) or the aim
// (the gunner), and the pull distance past a threshold is the throttle or
// the trigger. The pilot's heading is sent as a direction rather than the
// client deriving one, because the server has the stick and the heading at
// the same instant and the client would have to fight the stream's lag.
// The stick geometry, the aim geometry, and the poster are pure, so the
// browser suite evaluates them directly; only attach() touches the DOM.
//
// Loaded as a plain script in the browser; also require()-able in Node,
// so the helpers can be checked directly.
(function (root, factory) {
  const api = factory();
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  root.Controls = api;
})(globalThis, function () {
  "use strict";

  // Starting values from the settled scheme. The stick's throw is large
  // enough that the pull *distance* means something, which is what the
  // second stage needs; it is measured from the band's floor so it is the
  // same in portrait and landscape (see stickRadius).
  const DEAD_ZONE = 0.3; // fraction of the stick radius
  const THROTTLE_ZONE = 0.55; // past this the pilot also burns
  const FIRE_ZONE = 0.55; // past this the gunner also fires
  const STICK_RADIUS = 0.35; // fraction of the stick's reference size
  // STICK_FLOOR is the band's floor, `min-height: 140px` on #controls in
  // style.css. The band grows to fill the height the arena leaves, and
  // only where the stick may *appear* should follow it.
  const STICK_FLOOR = 140;

  // stickRadius is the stick's throw for a band of the given width. The
  // reference is the band's floor rather than the band's box: the band
  // grows to fill the height the arena leaves, and only where the stick
  // may *appear* should follow it — a stick scaled to the grown band
  // would lengthen the pull to fire. The width is still a bound so a
  // narrow band keeps the stick on screen.
  function stickRadius(width) {
    return STICK_RADIUS * Math.min(width, STICK_FLOOR);
  }

  // --- stick and aim geometry ---

  // shortestWrapped folds a delta into [-size/2, size/2), so a direction
  // is measured to the nearest copy across the arena seam.
  function shortestWrapped(delta, size) {
    const d = ((delta % size) + size) % size;
    return d >= size / 2 ? d - size : d;
  }

  // fitStick clamps a drag to the stick radius, so the knob stays on the
  // ring and a long drag keeps pointing the same way.
  function fitStick(dx, dy, radius) {
    const mag = Math.hypot(dx, dy);
    if (!mag || mag <= radius) return { dx, dy };
    const k = radius / mag;
    return { dx: dx * k, dy: dy * k };
  }

  // stickSteer reads the pilot's floating stick. The drag direction is
  // the heading the nose should point at, in the wire's convention (0
  // along +X), and the drag distance is the throttle: a light push steers
  // without spending fuel, a firm one also burns. Inside the dead zone
  // nothing is sent at all, so a resting thumb cannot keep the ship
  // turning.
  function stickSteer(dx, dy, radius) {
    const r = Math.hypot(dx, dy);
    if (r <= DEAD_ZONE * radius) return { steer: null, thrust: false };
    return { steer: aimFromDrag(dx, dy), thrust: r > THROTTLE_ZONE * radius };
  }

  // stickAim reads the gunner's floating stick the same way: the drag
  // direction is the turret's aim, in the wire's convention (0 along +X),
  // and the drag distance is the trigger — a light push lines up a shot
  // without spending it, a firm one also fires. Inside the dead zone
  // neither is sent, so the remembered aim stands and a resting thumb
  // cannot spin the gun. Because the aim is the drag *direction*, pushing
  // further along the same ray fires without moving the turret.
  function stickAim(dx, dy, radius) {
    const r = Math.hypot(dx, dy);
    if (r <= DEAD_ZONE * radius) return { aim: null, fire: false };
    return { aim: aimFromDrag(dx, dy), fire: r > FIRE_ZONE * radius };
  }

  // aimFromDrag is a direction in the wire's radians, 0 along +X, so "up"
  // on screen is -pi/2. Both sticks read a direction this way — the
  // gunner's aim and the pilot's steer — which is what keeps the turret
  // and the nose in one frame of reference.
  function aimFromDrag(dx, dy) {
    return Math.atan2(dy, dx);
  }

  // aimAngle is the absolute direction from the ship to an arena point,
  // measured across the seam so aiming at a wrapped copy points right.
  function aimAngle(ship, point, size) {
    return Math.atan2(
      shortestWrapped(point.y - ship.y, size),
      shortestWrapped(point.x - ship.x, size)
    );
  }

  // --- the poster ---

  // createPoster sends the current intent at once when it changes, so a
  // press is not held back, and again on every tick, so a held button
  // stays inside the server's freshness window. It can be paused while no
  // round is playing, and an urgent change (a release) skips the floor
  // that otherwise coalesces a fast drag. 401/404 mean the seat is gone
  // and 409 that the room is not ready; both stop the poster and let the
  // caller react, and a poster stopped by a 409 can be revived once the
  // room is ready again. set(), tick(), and the clock are all injected by
  // the caller, so the timing is testable.
  function createPoster(opts) {
    const {
      lobbyId,
      token,
      fetchImpl = globalThis.fetch.bind(globalThis),
      now = () => globalThis.performance.now(),
      minGapMs = 50,
      onGone = () => {},
      onNotReady = () => {},
    } = opts;

    const url = `/api/lobby/${encodeURIComponent(lobbyId)}/intent`;
    let seq = 0;
    let state = null;
    let lastPostAt = -Infinity;
    let stopped = false;
    let paused = false;

    function send() {
      if (stopped || paused || state === null) return;
      lastPostAt = now();
      seq += 1;
      const headers = { "Content-Type": "application/json" };
      if (token) headers["X-Player-Token"] = token;
      let res;
      try {
        res = fetchImpl(url, {
          method: "POST",
          headers,
          body: JSON.stringify({ seq, ...state }),
        });
      } catch {
        return;
      }
      if (!res || typeof res.then !== "function") return;
      res
        .then((r) => {
          if (r.status === 401 || r.status === 404) {
            stopped = true;
            onGone();
          } else if (r.status === 409) {
            stopped = true;
            onNotReady();
          }
        })
        .catch(() => {
          // A network error is transient: the next tick tries again.
        });
    }

    return {
      // set records the current intent. It posts at once unless the last
      // post was too recent, so a first press is not held back by the
      // 10 Hz tick; a release is urgent and skips even that floor, so the
      // ship stops at once.
      set(payload, urgent) {
        state = payload;
        if (stopped) return;
        if (urgent || now() - lastPostAt >= minGapMs) send();
      },
      // tick re-posts the current intent, keeping it fresh while held.
      tick() {
        send();
      },
      // pause holds posts back without forgetting the intent: the caller
      // pauses outside a playing phase and resumes when it begins, so a
      // control held across the boundary is not lost.
      pause() {
        paused = true;
      },
      resume() {
        if (!paused) return;
        paused = false;
        send();
      },
      stop() {
        stopped = true;
      },
      // revive undoes a stop caused by a 409, so a room that is ready
      // again accepts posts without a new poster. A poster stopped by
      // stop() or a gone seat is never revived: those paths drop it
      // instead of reviving it.
      revive() {
        stopped = false;
      },
      get seq() {
        return seq;
      },
    };
  }

  // --- DOM binding ---
  //
  // The band holds one floating stick per role (only one is ever shown)
  // and a shared base/knob pair. Everything is positioned against the
  // band, so a touch anywhere in it is in reach.

  function attach(opts) {
    const { band, stick, aim, base, knob, canvas, rest, restLabel, onIntent } = opts;

    let role = "";
    // The resting stand-in teaches the band once: it appears when a role
    // is rolled, and the first real pull takes it away for good so it
    // cannot flicker back between pulls.
    let restDismissed = false;
    let ship = null;
    let size = 1000;
    let mousePoint = null; // last mouse position over the arena, sim units
    let aimValue = null; // the remembered aim, radians
    let fireHeld = false;
    const keys = new Set();
    let lastPayload = "";

    // send hands a payload to the poster. A release is urgent: it has to
    // land at once rather than wait behind the floor that coalesces a
    // fast drag.
    function send(payload, urgent) {
      const json = JSON.stringify(payload);
      if (json === lastPayload) return;
      lastPayload = json;
      onIntent(payload, urgent === true);
    }

    // hideHint hides the floating base and knob, which both roles lose
    // when the finger lifts. The resting stand-in stays away once a pull
    // has taught the player where the control is.
    function hideHint() {
      base.hidden = true;
      knob.hidden = true;
      rest.hidden = !role || restDismissed;
    }

    function showHint(clientX, clientY, radius, dx, dy) {
      const r = band.getBoundingClientRect();
      // The live pad is here now: the stand-in has done its job, and it
      // does not come back between pulls.
      rest.hidden = true;
      restDismissed = true;
      base.hidden = false;
      base.style.width = `${radius * 2}px`;
      base.style.height = `${radius * 2}px`;
      base.style.left = `${clientX - r.left}px`;
      base.style.top = `${clientY - r.top}px`;
      knob.hidden = false;
      knob.style.left = `${clientX + dx - r.left}px`;
      knob.style.top = `${clientY + dy - r.top}px`;
    }

    // bindPad wires one floating stick. active() gates it to the role
    // that owns it; onEnd runs after the drag is dropped.
    function bindPad(el, active, onDrag, onEnd) {
      const pad = { drag: null };
      el.addEventListener("pointerdown", (e) => {
        if (!active() || pad.drag) return;
        e.preventDefault();
        el.setPointerCapture(e.pointerId);
        const r = el.getBoundingClientRect();
        const radius = stickRadius(r.width);
        pad.drag = {
          pointerId: e.pointerId,
          x: e.clientX,
          y: e.clientY,
          radius,
          dx: 0,
          dy: 0,
        };
        showHint(e.clientX, e.clientY, radius, 0, 0);
        onDrag(pad);
      });
      el.addEventListener("pointermove", (e) => {
        const d = pad.drag;
        if (!d || e.pointerId !== d.pointerId) return;
        const fit = fitStick(e.clientX - d.x, e.clientY - d.y, d.radius);
        d.dx = fit.dx;
        d.dy = fit.dy;
        showHint(d.x, d.y, d.radius, fit.dx, fit.dy);
        onDrag(pad);
      });
      const end = (e) => {
        const d = pad.drag;
        if (!d || (e && e.pointerId !== d.pointerId)) return;
        pad.drag = null;
        hideHint();
        onEnd();
      };
      for (const type of ["pointerup", "pointercancel", "lostpointercapture"]) {
        el.addEventListener(type, end);
      }
      return pad;
    }

    // --- the pilot ---

    const PILOT_KEYS = new Set(["arrowleft", "arrowright", "arrowup", "a", "d", "w"]);

    function keyIntent() {
      const left = keys.has("arrowleft") || keys.has("a");
      const right = keys.has("arrowright") || keys.has("d");
      const up = keys.has("arrowup") || keys.has("w");
      return { rotate: left === right ? 0 : left ? -1 : 1, thrust: up };
    }

    function pilotPayload() {
      const d = pilotPad.drag;
      const s = d ? stickSteer(d.dx, d.dy, d.radius) : { steer: null, thrust: false };
      const k = keyIntent();
      const payload = { thrust: s.thrust || k.thrust };
      // The stick names the heading to fly at; the keyboard can only pick
      // a side. The stick wins when both are held, which is the shape the
      // server prefers too.
      if (s.steer !== null) payload.steer = s.steer;
      else payload.rotate = k.rotate;
      return payload;
    }

    const pilotPad = bindPad(
      stick,
      () => role === "pilot",
      () => emit(),
      () => emit(true)
    );

    // --- the gunner ---

    function setAim(angle) {
      if (angle === aimValue) return;
      aimValue = angle;
      emit();
    }

    function setFire(on) {
      if (on === fireHeld) return;
      fireHeld = on;
      emit(true); // a press and a release both land at once
    }

    // The gunner holds one stick: its direction is the aim and its
    // distance is the trigger. The mouse and the space bar set fireHeld
    // instead, so a desktop player fires without the stick; the two paths
    // merge by OR and write the same wire fields.
    function gunnerPayload() {
      const d = aimPad.drag;
      const s = d ? stickAim(d.dx, d.dy, d.radius) : { aim: null, fire: false };
      const p = { fire: s.fire || fireHeld };
      const aim = s.aim !== null ? s.aim : aimValue;
      if (aim !== null) p.aim = aim;
      return p;
    }

    const aimPad = bindPad(
      aim,
      () => role === "gunner",
      (pad) => {
        const d = pad.drag;
        const s = stickAim(d.dx, d.dy, d.radius);
        // Past the dead zone the drag direction is the aim, and it is
        // remembered for after the finger lifts; inside it the remembered
        // aim stands, so a resting thumb does not spin the gun.
        if (s.aim !== null) {
          mousePoint = null;
          aimValue = s.aim;
        }
        emit();
      },
      () => emit(true) // the lift stops the fire at once; the aim stands
    );

    // --- keyboard, and the mouse over the arena ---

    function arenaPoint(e) {
      const r = canvas.getBoundingClientRect();
      if (!r.width) return null;
      const k = size / r.width;
      return { x: (e.clientX - r.left) * k, y: (e.clientY - r.top) * k };
    }

    addEventListener("keydown", (e) => {
      if (!role) return;
      const k = e.key.toLowerCase();
      if (role === "pilot" && PILOT_KEYS.has(k)) {
        e.preventDefault();
        keys.add(k);
        emit();
      } else if (role === "gunner" && k === " ") {
        e.preventDefault();
        setFire(true);
      }
    });
    addEventListener("keyup", (e) => {
      const k = e.key.toLowerCase();
      if (keys.delete(k)) emit(true); // a release lands at once
      else if (k === " ") setFire(false);
    });
    addEventListener("blur", () => {
      keys.clear();
      fireHeld = false;
      emit(true);
    });

    canvas.addEventListener("pointermove", (e) => {
      if (role !== "gunner" || e.pointerType !== "mouse") return;
      const p = arenaPoint(e);
      if (p) {
        mousePoint = p;
        refresh();
      }
    });
    canvas.addEventListener("pointerdown", (e) => {
      if (role !== "gunner" || e.pointerType !== "mouse") return;
      setFire(true);
    });
    addEventListener("pointerup", (e) => {
      if (e.pointerType === "mouse" && role === "gunner" && fireHeld) setFire(false);
    });

    // --- the published state ---

    function emit(urgent) {
      if (!role) return;
      send(role === "pilot" ? pilotPayload() : gunnerPayload(), urgent);
    }

    // refresh re-derives the mouse aim, which is absolute and so follows
    // the ship even when the pointer is still. The touch aim is relative
    // to its own origin and does not.
    function refresh() {
      if (role !== "gunner" || !mousePoint || !ship) return;
      setAim(aimAngle(ship, mousePoint, size));
    }

    return {
      // setRole follows the round's roll, swaps the visible controls, and
      // drops any input the previous role was holding.
      setRole(next) {
        if (next === role) return;
        role = next;
        keys.clear();
        fireHeld = false;
        aimValue = null;
        mousePoint = null;
        lastPayload = "";
        hideHint();
        // The caption is this role's instruction: the pilot pulls to move
        // the ship, the gunner pulls to fire. A roll is a new control to
        // learn, so the stand-in comes back with its own caption.
        restLabel.textContent =
          next === "pilot" ? "Pull to move" : next === "gunner" ? "Pull to fire" : "";
        restDismissed = false;
        rest.hidden = !next;
        stick.hidden = next !== "pilot";
        aim.hidden = next !== "gunner";
        emit(true);
      },
      // setView keeps the latest interpolated view, for the mouse aim and
      // the arena size.
      setView(view) {
        if (!view) return;
        if (view.arena) size = view.arena;
        ship = view.ship || null;
      },
      refresh,
      // live reports whether the player is holding something, so the
      // caller only posts while an intent has to stay fresh.
      live() {
        if (!role) return false;
        if (role === "pilot") {
          const k = keyIntent();
          return k.rotate !== 0 || k.thrust || pilotPad.drag !== null;
        }
        return fireHeld || aimValue !== null;
      },
    };
  }

  return {
    DEAD_ZONE,
    THROTTLE_ZONE,
    FIRE_ZONE,
    STICK_RADIUS,
    STICK_FLOOR,
    attach,
    fitStick,
    stickRadius,
    stickSteer,
    stickAim,
    aimFromDrag,
    aimAngle,
    shortestWrapped,
    createPoster,
  };
});
