// Input for the two copilots, plus the poster that sends what it produces.
//
// The scheme is settled in tasks/closed/define-touch-control-scheme.md: a
// floating stick for the pilot, a floating aim stick and a fire button for
// the gunner, and the same intents from a keyboard and mouse. The
// quantization, the aim geometry, and the poster are pure, so the browser
// suite evaluates them directly; only attach() touches the DOM.
//
// Loaded as a plain script in the browser; also require()-able in Node,
// so the helpers can be checked directly.
(function (root, factory) {
  const api = factory();
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  root.Controls = api;
})(globalThis, function () {
  "use strict";

  // Starting values from the settled scheme.
  const DEAD_ZONE = 0.3; // fraction of the stick radius
  const STICK_RADIUS = 0.15; // fraction of the zone's shorter side

  // --- quantization and aim geometry ---

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

  // quantizeStick turns a drag offset into the contract's discrete pilot
  // intent. The screen axes and the sim axes agree, so up is -y. A
  // diagonal drag turns and thrusts at once.
  function quantizeStick(dx, dy, radius) {
    const t = DEAD_ZONE * radius;
    return {
      rotate: dx < -t ? -1 : dx > t ? 1 : 0,
      thrust: dy < -t,
    };
  }

  // aimFromDrag is the gunner's relative aim: the drag direction from the
  // touch origin, in the wire's radians with 0 along +X.
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
  // caller react. set(), tick(), and the clock are all injected by the
  // caller, so the timing is testable.
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
      get seq() {
        return seq;
      },
    };
  }

  // --- DOM binding ---
  //
  // The band holds one floating stick per role (only one is ever shown),
  // a shared base/knob pair, and the gunner's fire button. Everything is
  // positioned against the band, so a touch anywhere in it is in reach.

  function attach(opts) {
    const { band, stick, aim, base, knob, fire, canvas, onIntent } = opts;

    let role = "";
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

    // hideHint hides the floating base and knob, which the pilot loses on
    // release and the gunner loses when the finger lifts.
    function hideHint() {
      base.hidden = true;
      knob.hidden = true;
    }

    function showHint(clientX, clientY, radius, dx, dy) {
      const r = band.getBoundingClientRect();
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
        const radius = STICK_RADIUS * Math.min(r.width, r.height);
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
      const s = quantizeStick(d ? d.dx : 0, d ? d.dy : 0, d ? d.radius : 1);
      const k = keyIntent();
      return { rotate: s.rotate || k.rotate, thrust: s.thrust || k.thrust };
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

    function gunnerPayload() {
      const p = { fire: fireHeld };
      if (aimValue !== null) p.aim = aimValue;
      return p;
    }

    const aimPad = bindPad(
      aim,
      () => role === "gunner",
      (pad) => {
        const d = pad.drag;
        // Past the dead zone the drag direction is the aim; inside it the
        // remembered aim stands, so a resting thumb does not spin the gun.
        if (Math.hypot(d.dx, d.dy) > DEAD_ZONE * d.radius) {
          mousePoint = null;
          setAim(aimFromDrag(d.dx, d.dy));
        }
      },
      () => {
        // The aim is remembered after the finger lifts: nothing to send.
      }
    );

    fire.addEventListener("pointerdown", (e) => {
      if (role !== "gunner") return;
      e.preventDefault();
      fire.setPointerCapture(e.pointerId);
      setFire(true);
    });
    for (const type of ["pointerup", "pointercancel", "lostpointercapture"]) {
      fire.addEventListener(type, () => setFire(false));
    }

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
        stick.hidden = next !== "pilot";
        aim.hidden = next !== "gunner";
        fire.hidden = next !== "gunner";
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
    STICK_RADIUS,
    attach,
    fitStick,
    quantizeStick,
    aimFromDrag,
    aimAngle,
    shortestWrapped,
    createPoster,
  };
});
