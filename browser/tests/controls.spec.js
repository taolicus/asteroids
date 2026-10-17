// Browser tests for the controls and the layout that holds them.
//
// The quantization, the aim geometry, and the poster are driven through
// the page's own Controls module, so these test the shipped code. The
// end-to-end cases seat two players, drive the pads, and read the intent
// posts the server actually receives.
import { test, expect, shareLink } from "./fixtures.js";

// ready plays a lobby through to a live round and returns the two pages
// keyed by the role each was rolled.
async function ready(page, players) {
  await page.goto("/");
  const link = await shareLink(page);

  const guest = await players();
  await guest.page.goto(link);

  const byRole = {};
  for (const p of [page, guest.page]) {
    // A live survival timer is what says the round is playing.
    await expect(p.locator("#hud-time")).not.toHaveText("0.0");
    const role = await p.locator("#hud-role").textContent();
    byRole[role.toLowerCase()] = p;
  }
  return byRole;
}

// intents collects the intent posts a page sends, in order.
function intents(page) {
  const seen = [];
  page.on("request", (request) => {
    if (request.method() !== "POST" || !request.url().includes("/intent")) return;
    try {
      seen.push(request.postDataJSON());
    } catch {
      // Not JSON: leave it out; the assertions below are about the body.
    }
  });
  return seen;
}

// last polls until the newest intent post matches, which is how a release
// is told apart from the drag that preceded it.
async function expectLast(posts, want, message) {
  await expect
    .poll(
      () => {
        const last = posts[posts.length - 1];
        return Boolean(last) && Object.entries(want).every(([k, v]) => last[k] === v);
      },
      { message },
    )
    .toBe(true);
}

test.describe("the control arithmetic", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("reads the pilot's stick as a heading and a throttle", async ({ page }) => {
    const out = await page.evaluate(() => ({
      rest: Controls.stickSteer(0, 0, 100),
      insideDeadZone: Controls.stickSteer(29, 0, 100),
      lightUp: Controls.stickSteer(0, -40, 100),
      firmUp: Controls.stickSteer(0, -60, 100),
      left: Controls.stickSteer(-60, 0, 100),
      diagonal: Controls.stickSteer(40, -40, 100),
      down: Controls.stickSteer(0, 60, 100),
    }));
    expect(out.rest).toEqual({ steer: null, thrust: false });
    expect(out.insideDeadZone).toEqual({ steer: null, thrust: false });
    // Straight up is -pi/2 in the wire's convention, whatever the ship is
    // doing, so the steer never depends on the heading.
    expect(out.lightUp.steer).toBeCloseTo(-Math.PI / 2, 9);
    expect(out.lightUp.thrust).toBe(false); // a light push steers only
    expect(out.firmUp.steer).toBeCloseTo(-Math.PI / 2, 9);
    expect(out.firmUp.thrust).toBe(true); // a firm push also burns
    expect(out.left.steer).toBeCloseTo(Math.PI, 9);
    expect(out.diagonal.steer).toBeCloseTo(-Math.PI / 4, 9);
    expect(out.diagonal.thrust).toBe(true);
    expect(out.down.steer).toBeCloseTo(Math.PI / 2, 9);
    expect(out.down.thrust).toBe(true);
  });

  test("keeps a long drag pointing the same way", async ({ page }) => {
    const fit = await page.evaluate(() => Controls.fitStick(300, 400, 100));
    expect(Math.hypot(fit.dx, fit.dy)).toBeCloseTo(100, 6);
    expect(fit.dx).toBeCloseTo(60, 6);
    expect(fit.dy).toBeCloseTo(80, 6);
  });

  test("takes the gunner's aim from the drag, and the mouse aim across the seam", async ({
    page,
  }) => {
    const out = await page.evaluate(() => ({
      right: Controls.aimFromDrag(50, 0),
      up: Controls.aimFromDrag(0, -50),
      left: Controls.aimFromDrag(-50, 0),
      aroundTheSeam: Controls.aimAngle({ x: 990, y: 500 }, { x: 10, y: 500 }, 1000),
      below: Controls.aimAngle({ x: 500, y: 500 }, { x: 500, y: 600 }, 1000),
    }));
    expect(out.right).toBeCloseTo(0, 6);
    expect(out.up).toBeCloseTo(-Math.PI / 2, 6);
    expect(out.left).toBeCloseTo(Math.PI, 6);
    expect(out.aroundTheSeam).toBeCloseTo(0, 6);
    expect(out.below).toBeCloseTo(Math.PI / 2, 6);
  });

  test("posts a change at once, then keeps the held intent fresh", async ({ page }) => {
    const out = await page.evaluate(() => {
      const posts = [];
      let clock = 1000;
      const poster = Controls.createPoster({
        lobbyId: "abcdefghijkl",
        now: () => clock,
        fetchImpl: (url, opts) => {
          posts.push({ url, ...JSON.parse(opts.body) });
          return Promise.resolve({ status: 200 });
        },
      });

      poster.set({ rotate: 1, thrust: false });
      const afterFirstSet = posts.length;
      clock += 10;
      poster.set({ rotate: 0, thrust: false }); // inside the coalescing floor
      const afterQuickChange = posts.length;
      clock += 100;
      poster.tick(); // the 10 Hz tick carries the newer intent
      return { afterFirstSet, afterQuickChange, posts };
    });

    expect(out.afterFirstSet).toBe(1);
    expect(out.afterQuickChange).toBe(1);
    expect(out.posts.map((p) => p.seq)).toEqual([1, 2]);
    expect(out.posts[1]).toMatchObject({ rotate: 0, thrust: false });
    expect(out.posts[0].url).toBe("/api/lobby/abcdefghijkl/intent");
  });

  test("a release skips the coalescing floor", async ({ page }) => {
    const posts = await page.evaluate(() => {
      const seen = [];
      let clock = 1000;
      const poster = Controls.createPoster({
        lobbyId: "abcdefghijkl",
        now: () => clock,
        fetchImpl: (url, opts) => {
          seen.push(JSON.parse(opts.body));
          return Promise.resolve({ status: 200 });
        },
      });
      poster.set({ rotate: 1, thrust: true });
      clock += 10;
      poster.set({ rotate: 0, thrust: false }, true);
      return seen;
    });
    expect(posts.map((p) => p.seq)).toEqual([1, 2]);
    expect(posts[1]).toMatchObject({ rotate: 0, thrust: false });
  });

  test("stops posting when the seat is gone", async ({ page }) => {
    const out = await page.evaluate(async () => {
      let posts = 0;
      let gone = 0;
      const poster = Controls.createPoster({
        lobbyId: "abcdefghijkl",
        now: () => 0,
        fetchImpl: () => {
          posts += 1;
          return Promise.resolve({ status: 401 });
        },
        onGone: () => {
          gone += 1;
        },
      });
      poster.set({ fire: true });
      await Promise.resolve(); // let the 401 land
      poster.set({ fire: false });
      poster.tick();
      return { posts, gone };
    });
    expect(out.gone).toBe(1);
    expect(out.posts).toBe(1);
  });
});

test.describe("the control band", () => {
  test("shows each copilot only their own controls", async ({ page, players }) => {
    const roles = await ready(page, players);

    await expect(roles.pilot.locator("#stick")).toBeVisible();
    await expect(roles.pilot.locator("#fire")).toBeHidden();
    await expect(roles.pilot.locator("#aim")).toBeHidden();

    await expect(roles.gunner.locator("#aim")).toBeVisible();
    await expect(roles.gunner.locator("#fire")).toBeVisible();
    await expect(roles.gunner.locator("#stick")).toBeHidden();
  });

  test("the pilot's stick posts a heading and a throttle, and the release posts neutral", async ({
    page,
    players,
  }) => {
    const roles = await ready(page, players);
    const pilot = roles.pilot;
    const posts = intents(pilot);

    const box = await pilot.locator("#stick").boundingBox();
    const cx = box.x + box.width / 2;
    const cy = box.y + box.height / 2;
    // The zones are fractions of the stick radius, which is a fraction of
    // the band's shorter side, so push by those fractions rather than by
    // pixels that only fit one viewport.
    const push = await pilot.evaluate(() => {
      const r = document.getElementById("stick").getBoundingClientRect();
      const radius = Controls.STICK_RADIUS * Math.min(r.width, r.height);
      return {
        light: ((Controls.DEAD_ZONE + Controls.THROTTLE_ZONE) / 2) * radius,
        firm: ((Controls.THROTTLE_ZONE + 1) / 2) * radius,
      };
    });

    // Straight up is -pi/2 on the wire; a light push steers without fuel.
    await pilot.mouse.move(cx, cy);
    await pilot.mouse.down();
    await pilot.mouse.move(cx, cy - push.light, { steps: 2 });
    await expect
      .poll(
        () => {
          const last = posts[posts.length - 1];
          return Boolean(last) && Math.abs(last.steer + Math.PI / 2) < 1e-6 && last.thrust === false;
        },
        { message: "a light push reaches the server as a heading, without burning" },
      )
      .toBe(true);

    // Pushing further is past the throttle zone, so it also burns.
    await pilot.mouse.move(cx, cy - push.firm, { steps: 4 });
    await expect
      .poll(
        () => {
          const last = posts[posts.length - 1];
          return Boolean(last) && Math.abs(last.steer + Math.PI / 2) < 1e-6 && last.thrust === true;
        },
        { message: "a firm push reaches the server as a heading and a burn" },
      )
      .toBe(true);

    await pilot.mouse.up();
    await expectLast(posts, { rotate: 0, thrust: false }, "the release reaches the server neutral");
  });

  test("the gunner aims, fires, and keeps the aim after the finger lifts", async ({
    page,
    players,
  }) => {
    const roles = await ready(page, players);
    const gunner = roles.gunner;
    const posts = intents(gunner);

    // Drag straight up in the aim band: the aim is -pi/2.
    const box = await gunner.locator("#aim").boundingBox();
    const cx = box.x + box.width / 2;
    const cy = box.y + box.height / 2;
    await gunner.mouse.move(cx, cy);
    await gunner.mouse.down();
    await gunner.mouse.move(cx, cy - 60, { steps: 4 });
    await expect
      .poll(() => posts.some((p) => Math.abs(p.aim + Math.PI / 2) < 1e-9), {
        message: "the aim reaches the server in radians",
      })
      .toBe(true);

    // The aim is remembered: firing after the lift carries it.
    await gunner.mouse.up();
    const fire = await gunner.locator("#fire").boundingBox();
    await gunner.mouse.move(fire.x + fire.width / 2, fire.y + fire.height / 2);
    await gunner.mouse.down();
    await expect
      .poll(() => posts.some((p) => p.fire === true && Math.abs(p.aim + Math.PI / 2) < 1e-9), {
        message: "fire carries the remembered aim",
      })
      .toBe(true);

    await gunner.mouse.up();
    await expectLast(posts, { fire: false }, "the release stops firing");
  });

  test("the pilot can also fly from the keyboard", async ({ page, players }) => {
    const roles = await ready(page, players);
    const posts = intents(roles.pilot);

    await roles.pilot.keyboard.down("ArrowLeft");
    await expect
      .poll(() => posts.some((p) => p.rotate === -1), { message: "the key reaches the server" })
      .toBe(true);

    await roles.pilot.keyboard.up("ArrowLeft");
    await expectLast(posts, { rotate: 0 }, "the key release is neutral");
  });
});

test.describe("the control band layout", () => {
  test("fits a phone viewport with no scrolling and a square arena", async ({ page, players }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await ready(page, players);

    const phone = await page.evaluate(() => {
      const arena = document.getElementById("arena").getBoundingClientRect();
      const band = document.getElementById("controls").getBoundingClientRect();
      const scroll = document.scrollingElement;
      return {
        arenaWidth: arena.width,
        arenaHeight: arena.height,
        arenaBottom: arena.bottom,
        bandTop: band.top,
        bandBottom: band.bottom,
        bandHeight: band.height,
        innerHeight,
        innerWidth,
        scrollHeight: scroll.scrollHeight,
        scrollWidth: scroll.scrollWidth,
      };
    });

    expect(Math.abs(phone.arenaWidth - phone.arenaHeight)).toBeLessThan(1);
    expect(phone.bandHeight).toBeGreaterThanOrEqual(140);
    expect(phone.arenaBottom).toBeLessThanOrEqual(phone.bandTop + 1);
    expect(phone.bandBottom).toBeLessThanOrEqual(phone.innerHeight + 1);
    expect(phone.scrollHeight).toBeLessThanOrEqual(phone.innerHeight + 1);
    expect(phone.scrollWidth).toBeLessThanOrEqual(phone.innerWidth + 1);
  });

  test("also fits a landscape viewport", async ({ page, players }) => {
    await page.setViewportSize({ width: 844, height: 390 });
    await ready(page, players);

    const landscape = await page.evaluate(() => {
      const arena = document.getElementById("arena").getBoundingClientRect();
      const band = document.getElementById("controls").getBoundingClientRect();
      const scroll = document.scrollingElement;
      return {
        arenaWidth: arena.width,
        arenaHeight: arena.height,
        bandBottom: band.bottom,
        innerHeight,
        scrollHeight: scroll.scrollHeight,
      };
    });

    expect(Math.abs(landscape.arenaWidth - landscape.arenaHeight)).toBeLessThan(1);
    expect(landscape.arenaWidth).toBeGreaterThan(0);
    expect(landscape.bandBottom).toBeLessThanOrEqual(landscape.innerHeight + 1);
    expect(landscape.scrollHeight).toBeLessThanOrEqual(landscape.innerHeight + 1);
  });
});
