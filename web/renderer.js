// Canvas drawing for the shared arena.
//
// Pure presentation: it draws a view produced by Interp (or any object
// with the same shape) and owns no state of its own beyond the canvas
// context. Wraparound entities are drawn in every copy that could be
// visible, so a ship crossing an edge slides out one side and in the
// other.
(function (root) {
  "use strict";

  const TWO_PI = Math.PI * 2;

  // Sizes in sim units. The projectile radius is a client constant (it is
  // not in the wire schema); the ship's shape is presentation only.
  const SHOT_RADIUS = 3;
  const SHIP_NOSE = 20;
  const SHIP_TAIL = 13;
  const SHIP_HALF_WIDTH = 9;
  // The barrel reaches past the hull so the aim reads even when the
  // gunner points back over the ship.
  const TURRET_LENGTH = 26;

  const COLORS = {
    background: "#0d1017",
    arena: "#232936",
    rock: "#8fa1c7",
    shot: "#ffd479",
    ship: "#e6e6e6",
    turret: "#ffd479",
  };

  function create(canvas) {
    const ctx = canvas.getContext("2d");

    // eachWrapped calls draw for every wrapped copy of (x, y) that can be
    // visible, so entities near a seam draw on both sides.
    function eachWrapped(x, y, size, margin, draw) {
      for (let ox = -1; ox <= 1; ox++) {
        for (let oy = -1; oy <= 1; oy++) {
          const cx = x + ox * size;
          const cy = y + oy * size;
          if (cx < -margin || cx > size + margin) continue;
          if (cy < -margin || cy > size + margin) continue;
          draw(cx, cy);
        }
      }
    }

    function render(view, nowMs) {
      const width = canvas.width;
      const height = canvas.height;
      const size = view.arena;
      // The arena is square and the canvas is square; scale to fit.
      const scale = Math.min(width, height) / size;
      const line = Math.max(1, 2 * scale);

      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.fillStyle = COLORS.background;
      ctx.fillRect(0, 0, width, height);

      // Arena boundary.
      ctx.strokeStyle = COLORS.arena;
      ctx.lineWidth = line;
      ctx.strokeRect(0, 0, size * scale, size * scale);

      // Rocks.
      ctx.strokeStyle = COLORS.rock;
      ctx.lineWidth = line;
      for (const rock of view.rocks) {
        eachWrapped(rock.x, rock.y, size, rock.radius + line, (x, y) => {
          ctx.beginPath();
          ctx.arc(x * scale, y * scale, Math.max(1, rock.radius * scale), 0, TWO_PI);
          ctx.stroke();
        });
      }

      // Projectiles.
      ctx.fillStyle = COLORS.shot;
      for (const shot of view.shots) {
        eachWrapped(shot.x, shot.y, size, SHOT_RADIUS + line, (x, y) => {
          ctx.beginPath();
          ctx.arc(x * scale, y * scale, Math.max(1, SHOT_RADIUS * scale), 0, TWO_PI);
          ctx.fill();
        });
      }

      // The shared ship, flashing while invulnerable.
      const ship = view.ship;
      if (ship) {
        const blinking = ship.invuln > 0 && Math.floor(nowMs / 120) % 2 === 0;
        ctx.globalAlpha = blinking ? 0.3 : 1;
        ctx.fillStyle = COLORS.ship;
        eachWrapped(ship.x, ship.y, size, SHIP_NOSE + line, (x, y) => {
          ctx.save();
          ctx.translate(x * scale, y * scale);
          ctx.rotate(ship.heading);
          ctx.beginPath();
          ctx.moveTo(SHIP_NOSE * scale, 0);
          ctx.lineTo(-SHIP_TAIL * scale, -SHIP_HALF_WIDTH * scale);
          ctx.lineTo(-SHIP_TAIL * scale, SHIP_HALF_WIDTH * scale);
          ctx.closePath();
          ctx.fill();
          ctx.restore();
        });
        // The gunner's turret, drawn from the centre so it reads
        // whichever way it points. A view without one falls back to the
        // nose, the sim's own default.
        const turret = typeof ship.turret === "number" ? ship.turret : ship.heading;
        ctx.strokeStyle = COLORS.turret;
        ctx.lineWidth = line;
        eachWrapped(ship.x, ship.y, size, TURRET_LENGTH + line, (x, y) => {
          ctx.save();
          ctx.translate(x * scale, y * scale);
          ctx.rotate(turret);
          ctx.beginPath();
          ctx.moveTo(0, 0);
          ctx.lineTo(TURRET_LENGTH * scale, 0);
          ctx.stroke();
          ctx.restore();
        });
        ctx.globalAlpha = 1;
      }
    }

    return { render };
  }

  root.Renderer = { create };
})(globalThis);
