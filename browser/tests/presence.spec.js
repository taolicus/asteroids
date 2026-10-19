// Browser tests for what a player is told when the other one leaves.
//
// A real absence takes 15 s of presence TTL plus the 5 s presence tick,
// so these cases serve canned SSE through page.route instead: the frames
// are exactly what stream.go writes, and everything downstream of the
// stream is the shipped client. The stream's production of those frames,
// and the leave beacon behind them, are covered by the Go tests.
import { test, expect, shareLink } from "./fixtures.js";

// frame renders one SSE event the way the server writes it.
function frame(event, data) {
  return `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
}

// playingFrame is a live round: the field exists and the timer runs.
function playingFrame(seq) {
  return frame("snapshot", {
    seq,
    tick: seq * 2,
    arena: 1000,
    phase: "playing",
    round: 1,
    pilot: "owner",
    lives: 3,
    score: 10,
    time: 4.2,
    ship: { x: 500, y: 500, heading: 0, turret: 0, invuln: 0 },
    rocks: [],
    shots: [],
  });
}

// overFrame is game over: the panel is up and a replay is on offer.
function overFrame(seq) {
  return frame("snapshot", {
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
    rocks: [],
    shots: [],
  });
}

const presenceFrame = (present) => frame("presence", { present });

// seated plays the owner into a round whose stream is the canned body.
// The guest is real, because a ready lobby is what opens the stream.
async function seated(page, players, body) {
  await page.goto("/");
  const link = await shareLink(page);
  await page.route("**/events", (route) =>
    route.fulfill({ status: 200, contentType: "text/event-stream", body }),
  );
  const guest = await players();
  await guest.page.goto(link);
  return guest;
}

test("a player whose friend left keeps playing and is told", async ({ page, players }) => {
  await seated(page, players, playingFrame(1) + presenceFrame(false));

  await expect(page.locator("#friend-note")).toBeVisible();
  await expect(page.locator("#friend-note")).toHaveText("Friend disconnected");
  await expect(page.locator("#arena")).toBeVisible();
  await expect(page.locator("#lobby")).toBeHidden();

  // Pressing a control sends an intent the server refuses (the room is
  // not ready) without dropping the player back to the lobby: the round
  // plays on solo.
  await page.route("**/intent", (route) =>
    route.fulfill({
      status: 409,
      contentType: "application/json",
      body: '{"error":"lobby is not ready"}',
    }),
  );
  const box = await page.locator("#stick").boundingBox();
  const [posted] = await Promise.all([
    page.waitForRequest((r) => r.method() === "POST" && r.url().includes("/intent")),
    page.mouse.move(box.x + box.width / 2, box.y + box.height / 2).then(() => page.mouse.down()),
  ]);
  expect(posted).toBeTruthy();

  await expect(page.locator("#arena")).toBeVisible();
  await expect(page.locator("#lobby")).toBeHidden();
  await expect(page.locator("#friend-note")).toBeVisible();
  await page.mouse.up();
});

test("a game over with the friend gone waits instead of offering a dead button", async ({
  page,
  players,
}) => {
  await seated(page, players, overFrame(1) + presenceFrame(false));

  await expect(page.locator("#round-title")).toHaveText("Game over");
  // Play again needs both players, so it is replaced by what is actually
  // happening rather than 409ing into nothing.
  await expect(page.locator("#friend-wait")).toBeVisible();
  await expect(page.locator("#friend-wait")).toHaveText(/Waiting for your friend/);
  await expect(page.locator("#play-again")).toBeHidden();
  // The panel tells the story, so the over-arena badge stays out of it.
  await expect(page.locator("#friend-note")).toBeHidden();
});

test("the replay button comes back when the friend does", async ({ page, players }) => {
  await seated(
    page,
    players,
    overFrame(1) + presenceFrame(false) + presenceFrame(true),
  );

  await expect(page.locator("#round-title")).toHaveText("Game over");
  await expect(page.locator("#play-again")).toBeVisible();
  await expect(page.locator("#friend-wait")).toBeHidden();
  await expect(page.locator("#friend-note")).toBeHidden();
});
