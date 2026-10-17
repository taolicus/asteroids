// Browser tests for the lobby.
//
// The `players` fixture and `shareLink` helper are shared with the
// renderer tests; see fixtures.js for how a "second browser" is a second
// isolated context.
//
// The panel itself is only the share link, so what the owner and the guest
// can be told apart by here is the link they carry, not any copy: the
// transition into a round is covered by the renderer and controls specs.
import { test, expect, shareLink } from "./fixtures.js";

test("index page is the owner's lobby, not a create screen", async ({ page }) => {
  await page.goto("/");

  await expect(page.locator("#lobby")).toBeVisible();
  await expect(page.locator("#share .hint")).toHaveText("Share your link with a friend:");
  // The panel carries no ownership copy to drift out of date.
  await expect(page.locator("#seat")).toHaveCount(0);
  await expect(page.locator("#state")).toHaveCount(0);
  await expect(page.locator("#error")).toBeHidden();
  await shareLink(page);

  await expect(page.getByRole("button", { name: /create/i })).toHaveCount(0);
});

test("the landing heading is the two-line title in the display face", async ({ page }) => {
  await page.goto("/");

  // innerText carries the CSS uppercase, and Playwright normalizes the
  // line break away, so the anchored pattern is what says the title is
  // two lines and has no comma.
  const heading = page.locator("h1");
  await expect(heading).toHaveText(/^🚀\s*two idiots\s*one ship$/i);
  await expect(heading).toHaveCSS("text-transform", "uppercase");

  // The title face is vendored and embedded, so it must actually load: a
  // misspelled file or a missing go:embed entry would only fall back to
  // the system stack.
  await expect
    .poll(() =>
      page.evaluate(() =>
        [...document.fonts].some((f) => f.family === "Michroma" && f.status === "loaded"),
      ),
    )
    .toBe(true);
});

test("the lobby says it is waiting, with a stepping ellipsis", async ({ page }) => {
  await page.goto("/");

  await expect(page.locator("#waiting")).toContainText("Waiting for friend");
  await expect(page.locator("#waiting-dots")).toHaveText(/^\.{1,3}$/);

  // The dots step one, two, three, and round again: sample until all three
  // have been seen rather than trusting one reading.
  const seen = new Set();
  for (let i = 0; i < 50 && seen.size < 3; i++) {
    seen.add(await page.locator("#waiting-dots").textContent());
    await page.waitForTimeout(100);
  }
  expect([...seen].sort()).toEqual([".", "..", "..."]);
});

test("the identity and its link survive a reload", async ({ page }) => {
  await page.goto("/");
  const before = await shareLink(page);

  await page.reload();

  await expect(page.locator("#share")).toBeVisible();
  expect(await shareLink(page)).toBe(before);
});

test("a guest entering the link gets the same room and the same link", async ({ page, players }) => {
  await page.goto("/");
  const link = await shareLink(page);

  const guest = await players();
  await guest.page.goto(link);

  await expect(guest.page.locator("#share .hint")).toHaveText("Share your link with a friend:");
  // The guest may already be playing, so read the panel's contents rather
  // than asserting the panel is on screen.
  await expect(guest.page.locator("#share-link")).toHaveValue(link);
});

test("a third player is told the room is full", async ({ page, players }) => {
  await page.goto("/");
  const link = await shareLink(page);

  const guest = await players();
  await guest.page.goto(link);
  // The room is ready once the guest is in, so its role appearing is the
  // guest having filled the seat.
  await expect(guest.page.locator("#hud-role")).toHaveText(/^(Pilot|Gunner)$/);

  const third = await players();
  await third.page.goto(link);

  await expect(third.page.locator("#error")).toBeVisible();
  await expect(third.page.locator("#error-message")).toHaveText("This lobby is already full.");
});

test("a malformed link shows a readable error", async ({ page }) => {
  await page.goto("/?r=not-a-lobby");

  await expect(page.locator("#error")).toBeVisible();
  await expect(page.locator("#error-message")).toHaveText("This link doesn't point to a lobby.");
  // The lobby view must be gone, not shown behind the error.
  await expect(page.locator("#lobby")).toBeHidden();
});

test("copy puts the share link on the clipboard", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/");
  const link = await shareLink(page);

  await page.locator("#copy").click();

  await expect(page.locator("#copy")).toHaveText("Copied!");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(link);
});
