// Browser tests for the lobby.
//
// The `players` fixture and `shareLink` helper are shared with the
// renderer tests; see fixtures.js for how a "second browser" is a second
// isolated context.
//
// The owner's panel is the share link and the waiting line; a guest in
// someone else's room gets the away notice instead, so the two seats are
// told apart by which panel they get. The transition into a round is
// covered by the renderer and controls specs.
import { test, expect, shareLink } from "./fixtures.js";

test("index page is the owner's lobby, not a create screen", async ({ page }) => {
  await page.goto("/");

  await expect(page.locator("#lobby")).toBeVisible();
  await expect(page.locator("#share .hint")).toHaveText("Share your link with a friend:");
  // The landing page is the player's own room: not the away notice, and
  // not a create screen.
  await expect(page.locator("#guest")).toBeHidden();
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

test("the lobby shows a waiting spinner while it waits", async ({ page }) => {
  await page.goto("/");

  await expect(page.locator("#waiting")).toContainText("Waiting for friend");
  const spinner = page.locator("#spinner");
  await expect(spinner).toBeVisible();
  await expect(spinner).toHaveCSS("animation-name", "spin");
  await expect(page.locator("#waiting-dots")).toHaveCount(0);
});

test("the identity and its link survive a reload", async ({ page }) => {
  await page.goto("/");
  const before = await shareLink(page);

  await page.reload();

  await expect(page.locator("#share")).toBeVisible();
  expect(await shareLink(page)).toBe(before);
});

test("a guest opening an away room is told, and can go to their own", async ({ players }) => {
  // A well-formed link whose owner has never been here: the room exists
  // lazily and holds this visitor alone, so the host is away by definition.
  const link = "/?r=bbbbbbbbbbbb";

  const guest = await players();
  await guest.page.goto(link);

  // The room is not theirs, so there is no share box: the page says what
  // is happening and waits.
  await expect(guest.page.locator("#guest")).toBeVisible();
  await expect(guest.page.locator("#guest-waiting")).toHaveText(/Waiting for your friend/);
  await expect(guest.page.locator("#guest-spinner")).toBeVisible();
  await expect(guest.page.locator("#lobby")).toBeHidden();
  await expect(guest.page.locator("#share")).toBeHidden();
  await expect(guest.page.locator("#error")).toBeHidden();

  // Leaving lands on their own room, with their own permanent link, and
  // the other player's ?r= gone from the address bar.
  await guest.page.locator("#guest-leave").click();

  await expect(guest.page.locator("#lobby")).toBeVisible();
  await expect(guest.page.locator("#guest")).toBeHidden();
  await expect(guest.page.locator("#share")).toBeVisible();
  expect(guest.page.url()).not.toContain("?r=");
  await shareLink(guest.page);
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

  // Reloading would land on the same link and the same error; the way
  // out is the player's own room.
  await expect(third.page.locator("#home")).toBeVisible();
  await third.page.locator("#home").click();

  await expect(third.page.locator("#lobby")).toBeVisible();
  await expect(third.page.locator("#error")).toBeHidden();
  expect(await shareLink(third.page)).not.toBe(link);
});

test("a malformed link shows a readable error", async ({ page }) => {
  await page.goto("/?r=not-a-lobby");

  await expect(page.locator("#error")).toBeVisible();
  await expect(page.locator("#error-message")).toHaveText("This link doesn't point to a lobby.");
  // Neither waiting panel may be left behind the error.
  await expect(page.locator("#lobby")).toBeHidden();
  await expect(page.locator("#guest")).toBeHidden();
});

test("copy puts the share link on the clipboard", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/");
  const link = await shareLink(page);

  await page.locator("#copy").click();

  await expect(page.locator("#copy")).toHaveText("Copied!");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(link);
});
