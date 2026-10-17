// Shared Playwright fixtures and helpers for the browser suite.
//
// A second player is a separate browser context: contexts do not share
// localStorage, so they hold independent identities and seats against the
// same server.
import { test as base, expect } from "@playwright/test";

// Share links are permanent: /?r=<12-char lobby ID>.
export const LOBBY_LINK = /\/\?r=[A-Za-z0-9_-]{12}$/;

// The `players` fixture opens extra isolated players and closes them when
// the test ends.
export const test = base.extend({
  players: async ({ browser }, use) => {
    const contexts = [];
    const open = async () => {
      const context = await browser.newContext();
      contexts.push(context);
      return { context, page: await context.newPage() };
    };
    await use(open);
    for (const context of contexts) {
      await context.close();
    }
  },
});

// Reads the owner's share link, asserting the lobby view is up first.
export async function shareLink(page) {
  await expect(page.locator("#share")).toBeVisible();
  const link = await page.locator("#share-link").inputValue();
  expect(link).toMatch(LOBBY_LINK);
  return link;
}
