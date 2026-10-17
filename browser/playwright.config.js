// Playwright config for the lobby browser tests.
//
// The suite runs against the real server binary: Playwright starts it
// through `webServer`, so these tests exercise the same embedded
// frontend and API that production serves.
import { defineConfig } from "@playwright/test";

const PORT = process.env.PORT || 18099;
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: "./tests",
  // The lobby polls every 2s, so UI changes arrive on the next poll.
  expect: { timeout: 10_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: BASE_URL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: "go run ./cmd/server",
    cwd: "..",
    url: `${BASE_URL}/`,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    env: {
      ADDR: `127.0.0.1:${PORT}`,
      // A fixed test key keeps identities stable for the whole run; it
      // is never used in production.
      LOBBY_KEY: "browser-test-key",
    },
  },
  projects: [{ name: "chromium" }],
});
