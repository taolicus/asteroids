// Browser tests for the renderer.
//
// The interpolation cases drive the page's own Interp module through
// page.evaluate, so they test the shipped arithmetic rather than a copy.
// The end-to-end cases play a ready lobby in two contexts and inspect the
// real canvas and HUD.
import { test, expect, shareLink } from "./fixtures.js";

// snapshot builds a wire-shaped snapshot with sensible defaults; only the
// fields a case cares about need to be passed.
function snapshot(overrides = {}) {
  return {
    seq: 1,
    tick: 1,
    arena: 1000,
    phase: "playing",
    round: 1,
    pilot: "owner",
    lives: 3,
    score: 0,
    time: 0,
    ship: null,
    rocks: [],
    shots: [],
    ...overrides,
  };
}

// moving is a snapshot whose ship has advanced to x.
function moving(seq, x) {
  return snapshot({
    seq,
    tick: seq,
    time: seq,
    ship: { x, y: 0, heading: 0, turret: 0, invuln: 0 },
  });
}

test.describe("snapshot interpolation", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("renders about 100 ms behind", async ({ page }) => {
    const x = await page.evaluate(([first, second]) => {
      const b = new Interp.SnapshotBuffer({ delayMs: 100 });
      b.push(first, 1000);
      b.push(second, 1100);
      return b.sample(1150).ship.x; // renderAt = 1050, halfway
    }, [moving(1, 0), moving(2, 10)]);
    expect(x).toBeCloseTo(5, 6);
  });

  test("never extrapolates past the newest snapshot", async ({ page }) => {
    const x = await page.evaluate(([first, second]) => {
      const b = new Interp.SnapshotBuffer({ delayMs: 100 });
      b.push(first, 0);
      b.push(second, 100);
      return b.sample(100000).ship.x; // far past the newest
    }, [moving(1, 0), moving(2, 10)]);
    expect(x).toBeCloseTo(10, 6);
  });

  test("interpolates the short way around the wraparound", async ({ page }) => {
    const x = await page.evaluate(([first, second]) => {
      const b = new Interp.SnapshotBuffer({ delayMs: 0 });
      b.push(first, 0);
      b.push(second, 100);
      return b.sample(50).ship.x; // halfway across the seam
    }, [moving(1, 995), moving(2, 5)]);
    // 995 + 10 * 0.5 = 1000, which wraps to 0.
    expect(Math.min(x, 1000 - x)).toBeLessThan(1e-6);
  });

  test("drops duplicate and stale events", async ({ page }) => {
    const accepted = await page.evaluate((events) => {
      const b = new Interp.SnapshotBuffer();
      return events.map(([snap, at]) => b.push(snap, at));
    }, [
      [snapshot({ seq: 5 }), 0],
      [snapshot({ seq: 5 }), 10],
      [snapshot({ seq: 4 }), 20],
      [snapshot({ seq: 6 }), 30],
    ]);
    expect(accepted).toEqual([true, false, false, true]);
  });

  test("snaps instead of smearing across a long gap", async ({ page }) => {
    const x = await page.evaluate(([first, second]) => {
      const b = new Interp.SnapshotBuffer({ delayMs: 100 });
      // 400 ms apart: past the 250 ms gap, so jump to the newer (200),
      // not interpolate to ~175.
      b.push(first, 0);
      b.push(second, 400);
      return b.sample(450).ship.x;
    }, [moving(1, 0), moving(2, 200)]);
    expect(x).toBeCloseTo(200, 6);
  });

  test("snaps on a large jump (a respawn)", async ({ page }) => {
    const x = await page.evaluate(([first, second]) => {
      const b = new Interp.SnapshotBuffer({ delayMs: 100 });
      // One 67 ms interval, but a >300-unit jump: snap to the newer.
      b.push(first, 0);
      b.push(second, 67);
      return b.sample(150).ship.x;
    }, [moving(1, 200), moving(2, 800)]);
    expect(x).toBeCloseTo(800, 6);
  });

  test("pairs entities by id and drops removed ones", async ({ page }) => {
    const view = await page.evaluate(([first, second]) => {
      const b = new Interp.SnapshotBuffer({ delayMs: 100 });
      b.push(first, 0);
      b.push(second, 100);
      const v = b.sample(150); // halfway
      return {
        rock1x: v.rocks.find((r) => r.id === 1).x,
        rock3x: v.rocks.find((r) => r.id === 3).x,
        rockCount: v.rocks.length,
        shotCount: v.shots.length,
      };
    }, [
      snapshot({
        seq: 1,
        rocks: [
          { id: 1, x: 0, y: 0, radius: 30 },
          { id: 2, x: 500, y: 500, radius: 20 },
        ],
        shots: [{ id: 9, x: 100, y: 100 }],
      }),
      snapshot({
        seq: 2,
        tick: 2,
        rocks: [
          { id: 1, x: 100, y: 0, radius: 30 },
          { id: 3, x: 1, y: 1, radius: 15 },
        ],
        shots: [],
      }),
    ]);
    expect(view.rock1x).toBeCloseTo(50, 6); // interpolated
    expect(view.rock3x).toBeCloseTo(1, 6); // new: not moved
    expect(view.rockCount).toBe(2); // id 2 is gone
    expect(view.shotCount).toBe(0); // shot is gone
  });

  test("interpolates the turret independently of the heading", async ({ page }) => {
    const ship = await page.evaluate(([first, second]) => {
      const b = new Interp.SnapshotBuffer({ delayMs: 100 });
      b.push(first, 1000);
      b.push(second, 1100);
      return b.sample(1150).ship; // renderAt = 1050, halfway
    }, [
      snapshot({ seq: 1, ship: { x: 0, y: 0, heading: 0, turret: 0, invuln: 0 } }),
      snapshot({
        seq: 2,
        ship: { x: 0, y: 0, heading: 0, turret: Math.PI / 2, invuln: 0 },
      }),
    ]);
    expect(ship.turret).toBeCloseTo(Math.PI / 4, 6);
    expect(ship.heading).toBeCloseTo(0, 6); // the hull did not turn
  });
});

test.describe("the shared arena", () => {
  test("both players see the arena and a live HUD once ready", async ({ page, players }) => {
    await page.goto("/");
    const link = await shareLink(page);

    const guest = await players();
    await guest.page.goto(link);

    for (const p of [page, guest.page]) {
      await expect(p.locator("#arena")).toBeVisible();
      await expect(p.locator("#hud")).toBeVisible();
      await expect(p.locator("#hud-lives")).toHaveText("3");
      await expect(p.locator("#hud-score")).toHaveText("0");
    }

    // The survival timer advances, proving snapshots are streaming and the
    // render loop is consuming them.
    await expect(page.locator("#hud-time")).not.toHaveText("0.0");
    await expect(guest.page.locator("#hud-time")).not.toHaveText("0.0");
  });

  test("the canvas is actually drawn", async ({ page, players }) => {
    await page.goto("/");
    const link = await shareLink(page);

    const guest = await players();
    await guest.page.goto(link);
    await expect(page.locator("#hud-time")).not.toHaveText("0.0");

    // Count pixels that differ from the arena background (#0d1017). Rocks,
    // shots, or the ship must put some down.
    const drawn = await page.evaluate(() => {
      const canvas = document.getElementById("arena");
      const ctx = canvas.getContext("2d");
      if (!canvas.width || !canvas.height) return -1;
      const { data } = ctx.getImageData(0, 0, canvas.width, canvas.height);
      let n = 0;
      for (let i = 0; i < data.length; i += 4) {
        if (data[i] > 40 || data[i + 1] > 40 || data[i + 2] > 40) n++;
      }
      return n;
    });
    expect(drawn).toBeGreaterThan(100);
  });

  test("draws the turret wherever the gunner aims, not where the hull points", async ({ page }) => {
    await page.goto("/");

    // The hull always faces left (heading π) and only the aim changes. The
    // barrel is the projectile yellow, which no other element shares, so
    // counting yellow-ish pixels right of the centre shows where it went.
    const rightOf = (turret) =>
      page.evaluate((t) => {
        const canvas = document.createElement("canvas");
        canvas.width = canvas.height = 200;
        const renderer = Renderer.create(canvas);
        renderer.render(
          {
            arena: 1000,
            rocks: [],
            shots: [],
            ship: { x: 500, y: 500, heading: Math.PI, turret: t, invuln: 0 },
          },
          0,
        );
        const { data } = canvas.getContext("2d").getImageData(0, 0, 200, 200);
        let right = 0;
        for (let y = 0; y < 200; y++) {
          for (let x = 101; x < 200; x++) {
            const i = (y * 200 + x) * 4;
            // Yellow over any backdrop: green sits clearly above blue,
            // which the hull's grey and the arena's blue both fail.
            if (data[i + 1] - data[i + 2] > 15 && data[i + 1] > 60) right++;
          }
        }
        return right;
      }, turret);

    const aimingRight = await rightOf(0);
    const aimingLeft = await rightOf(Math.PI);
    expect(aimingRight).toBeGreaterThan(3);
    expect(aimingRight).toBeGreaterThan(aimingLeft);
  });

  test("each copilot is told their role before the round starts", async ({ page, players }) => {
    await page.goto("/");
    const link = await shareLink(page);

    const guest = await players();
    await guest.page.goto(link);

    // The guest enters the lobby and opens the first stream, so its round
    // opens in the announcement window, role and all.
    await expect(guest.page.locator("#round-title")).toBeVisible();
    await expect(guest.page.locator("#round-title")).toContainText("you're the");

    // The roll rides the shared snapshot; each page reads it against its
    // own seat, so the copilots are told opposite roles.
    await expect(page.locator("#hud-role")).toHaveText(/^(Pilot|Gunner)$/);
    await expect(guest.page.locator("#hud-role")).toHaveText(/^(Pilot|Gunner)$/);
    const ownerRole = await page.locator("#hud-role").textContent();
    const guestRole = await guest.page.locator("#hud-role").textContent();
    expect(ownerRole).not.toBe(guestRole);
  });
});

// The round screens end a round, and a real round cannot be driven to game
// over quickly: a stationary ship at the centre is only hit by chance as
// rocks drift past. This block therefore feeds the owner's page the
// snapshots the server would send at game over and drives the shipped
// panel code directly. Everything downstream of the stream is real.
test.describe("the round screens", () => {
  // overFrame is one game-over snapshot as the wire schema encodes it.
  function overFrame(seq) {
    return (
      `event: snapshot\nid: ${seq}\ndata: ` +
      JSON.stringify({
        seq,
        tick: seq * 2,
        arena: 1000,
        phase: "over",
        round: 3,
        pilot: "owner",
        lives: 0,
        score: 420,
        time: 61.5,
        ship: null,
        rocks: [{ id: 1, x: 100, y: 100, radius: 24 }],
        shots: [],
      }) +
      "\n\n"
    );
  }

  test("game over shows the final score and offers a replay", async ({ page, players }) => {
    await page.goto("/");
    const link = await shareLink(page);

    // Serve the owner canned snapshots instead of its real stream, so the
    // page lands on game over without waiting out a real round.
    await page.route("**/events", (route) =>
      route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: overFrame(1) + overFrame(2) + overFrame(3),
      }),
    );

    // A ready lobby is what opens the stream in the first place.
    const guest = await players();
    await guest.page.goto(link);

    await expect(page.locator("#round-title")).toHaveText("Game over");
    await expect(page.locator("#round-stats")).toContainText("Round 3");
    await expect(page.locator("#round-stats")).toContainText("Score 420");
    await expect(page.locator("#round-stats")).toContainText("61.5s");
    await expect(page.locator("#hud-role")).toHaveText("Pilot");
    await expect(page.locator("#play-again")).toBeVisible();

    // Play again asks the server for a fresh round.
    const [request] = await Promise.all([
      page.waitForRequest((r) => r.method() === "POST" && r.url().includes("/restart")),
      page.locator("#play-again").click(),
    ]);
    expect(request).toBeTruthy();
  });
});
