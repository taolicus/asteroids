// Browser tests for the lobby.
//
// The `players` fixture and `shareLink` helper are shared with the
// renderer tests; see fixtures.js for how a "second browser" is a second
// isolated context.
import { test, expect, shareLink } from "./fixtures.js";

test("index page is the owner's lobby, not a create screen", async ({ page }) => {
  await page.goto("/");

  await expect(page.locator("#lobby")).toBeVisible();
  await expect(page.locator("#seat")).toHaveText("You own this lobby.");
  await expect(page.locator("#state")).toHaveText("Waiting for your copilot to open the link.");
  await expect(page.locator("#error")).toBeHidden();
  await shareLink(page);

  await expect(page.getByRole("button", { name: /create/i })).toHaveCount(0);
});

test("the identity and its link survive a reload", async ({ page }) => {
  await page.goto("/");
  const before = await shareLink(page);

  await page.reload();

  await expect(page.locator("#seat")).toHaveText("You own this lobby.");
  expect(await shareLink(page)).toBe(before);
});

test("a guest opening the link joins as guest and both see ready", async ({ page, players }) => {
  await page.goto("/");
  const link = await shareLink(page);

  const guest = await players();
  await guest.page.goto(link);

  await expect(guest.page.locator("#seat")).toHaveText("You're the guest in this lobby.");
  await expect(guest.page.locator("#state")).toHaveText("The owner is here.");
  // The share affordance belongs to the owner alone.
  await expect(guest.page.locator("#share")).toBeHidden();

  // The owner's next poll sees the guest.
  await expect(page.locator("#state")).toHaveText("Your copilot is here.");
});

test("a third player sees the lobby as full", async ({ page, players }) => {
  await page.goto("/");
  const link = await shareLink(page);

  const guest = await players();
  await guest.page.goto(link);
  await expect(guest.page.locator("#seat")).toHaveText("You're the guest in this lobby.");

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
