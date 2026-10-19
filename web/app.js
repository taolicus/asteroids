const API = "/api";
const POLL_MS = 2000;
const POST_MS = 100; // the settled 10 Hz poster
const MAX_ARENA = 640; // the largest the arena is drawn
const IDENTITY_KEY = "asteroids-identity";
const RECONNECT_MIN_MS = 500;
const RECONNECT_MAX_MS = 5000;

const $ = (id) => document.getElementById(id);

// The arena canvas is the game surface. Its size comes from the stage the
// flex layout gives it, so the page never scrolls mid-round.
const canvas = $("arena");

// resize mirrors the arena's box into the canvas backing store. In the
// lobby the box is the stage's square; while a round runs the arena takes
// the largest square that still leaves the stats and the control band
// their space, because those must stay on screen and the band is what the
// player's thumbs land on. The band grows into whatever the arena leaves
// over, so it is its floor — not its current height — that the arena is
// measured against, and the round panel's height is part of the budget
// while it is up.
function resize() {
  const stage = $("stage");
  const main = stage.parentElement;
  const box = stage.getBoundingClientRect();
  let side = Math.min(box.width, box.height, MAX_ARENA);
  if (document.body.classList.contains("playing")) {
    const mainStyle = getComputedStyle(main);
    const padding = parseFloat(mainStyle.paddingTop) + parseFloat(mainStyle.paddingBottom);
    const band = $("controls");
    const floor = parseFloat(getComputedStyle(band).minHeight) || band.offsetHeight;
    const panel = $("round");
    const spare = main.clientHeight
      - $("hud").offsetHeight
      - floor
      - padding
      - (panel.hidden ? 0 : panel.offsetHeight);
    side = Math.min(box.width, spare, MAX_ARENA);
  }
  if (side > 0) {
    canvas.style.width = `${side}px`;
    canvas.style.height = `${side}px`;
  } else {
    canvas.style.width = "100%";
    canvas.style.height = "auto";
  }
  canvas.width = Math.round(canvas.clientWidth * devicePixelRatio);
  canvas.height = Math.round(canvas.clientHeight * devicePixelRatio);
}

addEventListener("resize", resize);
resize();

// --- storage helpers (private browsing may deny localStorage) ---

function read(key) {
  try {
    return JSON.parse(localStorage.getItem(key));
  } catch {
    return null;
  }
}

function write(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Storage unavailable: the identity or seat just won't survive a
    // reload. Everything still works for this page view.
  }
}

function remove(key) {
  try {
    localStorage.removeItem(key);
  } catch {
    // Nothing to do.
  }
}

// --- identity ---
//
// A player's pseudo-identity is global to the browser: a public lobby ID
// plus a secret owner credential, created on first visit and kept
// forever. The lobby link (?r=<lobbyId>) is permanent; the server
// recreates the ephemeral instance whenever it is used.

let identity = null;

async function ensureIdentity() {
  if (identity) return identity;
  const saved = read(IDENTITY_KEY);
  if (saved && saved.lobbyId && saved.token) {
    identity = saved;
    return identity;
  }
  const data = await fetchJSON(`${API}/identities`, { method: "POST" });
  identity = { lobbyId: data.lobbyId, token: data.token };
  write(IDENTITY_KEY, identity);
  return identity;
}

// A guest's reconnect token belongs to one room instance, so it is
// stored per lobby ID and can be discarded freely.
const guestKey = (lobbyId) => `asteroids-guest:${lobbyId}`;

function loadGuestToken(lobbyId) {
  return read(guestKey(lobbyId))?.token || "";
}

function saveGuestToken(lobbyId, token) {
  write(guestKey(lobbyId), { token });
}

// --- HTTP ---

async function fetchJSON(url, options) {
  const res = await fetch(url, options);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(data.error || `request failed (${res.status})`);
    err.status = res.status;
    throw err;
  }
  return data;
}

// --- lobby flow ---

// session is the lobby this page is looking at; game is the running
// render loop and snapshot stream, or null while waiting.
let session = null;
let pollTimer = null;
let game = null;


async function start() {
  let lobbyId;
  try {
    const me = await ensureIdentity();
    lobbyId = new URLSearchParams(location.search).get("r") || me.lobbyId;
  } catch {
    showError("Could not reach the server. Check your connection and reload.");
    return;
  }
  await openLobby(lobbyId);
}

// openLobby enters (or re-enters) a lobby. It is used for the first visit
// and to recover after an instance is reclaimed or a guest seat is lost.
async function openLobby(lobbyId) {
  stopGame();
  const me = identity;
  const ownLobby = lobbyId === me.lobbyId;
  const credential = ownLobby ? me.token : loadGuestToken(lobbyId);

  let data;
  try {
    data = await fetchJSON(`${API}/lobby/${encodeURIComponent(lobbyId)}`, {
      method: "POST",
      headers: credential ? { "X-Player-Token": credential } : {},
    });
  } catch (e) {
    showError(errorMessage(e));
    return;
  }

  if (data.seat === "guest" && data.token) {
    saveGuestToken(lobbyId, data.token);
  }
  session = {
    lobbyId,
    seat: data.seat,
    token: data.seat === "owner" ? me.token : data.token || credential,
  };
  render();

  // A ready lobby plays; otherwise keep polling for the second player.
  if (isReady(data)) startGame();
  else startPolling();
}

function isReady(data) {
  return data && data.state === "ready";
}

function startPolling() {
  clearInterval(pollTimer);
  pollTimer = setInterval(poll, POLL_MS);
}

async function poll() {
  if (!session) return;
  let data;
  try {
    data = await fetchJSON(`${API}/lobby/${encodeURIComponent(session.lobbyId)}`, {
      headers: session.token ? { "X-Player-Token": session.token } : {},
    });
  } catch (e) {
    // 404: the instance was reclaimed while nobody was connected.
    // 401: our guest seat was handed to someone else.
    // Either way, re-entering recreates or reclaims the seat.
    if (e.status === 404 || e.status === 401) {
      await openLobby(session.lobbyId);
    }
    // Transient network errors: keep polling.
    return;
  }
  render();
  if (isReady(data)) startGame();
}

// --- game: snapshot stream and render loop ---

// startGame opens the events stream and starts interpolating and drawing.
// It is idempotent; polling stops once the game is running, per the
// settled contract.
function startGame() {
  clearInterval(pollTimer);
  pollTimer = null;
  if (game) return;

  $("arena").hidden = false;
  $("hud").hidden = false;
  $("controls").hidden = false;
  $("round").hidden = true;
  // The lobby panel is the room link and the seat; a playing round does
  // not need it, and on a phone its height is the arena's.
  show("game");
  document.body.classList.add("playing");
  resize();

  const g = {
    lobbyId: session.lobbyId,
    token: session.token,
    buffer: new Interp.SnapshotBuffer(),
    renderer: Renderer.create(canvas),
    controller: null,
    poster: null,
    postTimer: 0,
    posting: false,
    raf: 0,
    stopped: false,
    connected: false,
    retryMs: 0,
    timer: 0,
    // The other seat's presence, as the stream reports it. The room was
    // ready to connect, so it starts true.
    opponentPresent: true,
  };
  game = g;
  g.poster = Controls.createPoster({
    lobbyId: g.lobbyId,
    token: g.token,
    onGone: () => {
      // The seat is gone: re-enter, exactly as the stream handler does.
      if (game !== g) return;
      stopGame();
      openLobby(g.lobbyId);
    },
    onNotReady: () => {
      // The room is not ready, which mid-round can only mean a seat is
      // absent. The round plays on solo and the stream's presence
      // events bring the poster back when the friend returns.
      if (game !== g) return;
      applyPresence(false, g);
    },
  });
  // The poster starts paused: nothing is sent until a round is playing and
  // the player is holding a control.
  g.poster.pause();
  g.raf = requestAnimationFrame(frame);
  connectStream();
}

function stopGame() {
  if (!game) return;
  game.stopped = true;
  cancelAnimationFrame(game.raf);
  clearTimeout(game.timer);
  clearInterval(game.postTimer);
  if (game.poster) game.poster.stop();
  if (game.controller) game.controller.abort();
  game = null;
  controls.setRole("");
  document.body.classList.remove("playing");
  $("arena").hidden = true;
  $("hud").hidden = true;
  $("controls").hidden = true;
  $("round").hidden = true;
  $("friend-note").hidden = true;
  // Back to the waiting panel for this seat; a poll or a re-entry fills it
  // in again.
  show(waitingView());
}

function frame(now) {
  const g = game;
  if (!g || g.stopped) return;

  const view = g.buffer.sample(now);
  if (view) {
    g.renderer.render(view, now);
    updateHud(view, g);
    controls.setView(view);
    controls.setRole(roleName(view).toLowerCase());
    controls.refresh();
    setPosting(view.phase === "playing" && controls.live());
  }
  g.raf = requestAnimationFrame(frame);
}

// setPosting runs the 10 Hz poster only while a round is playing and the
// player is holding something, so an idle client sends nothing at all and
// a held control never goes stale. Outside a round the poster is paused
// rather than stopped, so a control held across the boundary survives.
function setPosting(on) {
  const g = game;
  if (!g || g.posting === on) return;
  g.posting = on;
  if (!on) {
    clearInterval(g.postTimer);
    g.postTimer = 0;
    g.poster.pause();
    return;
  }
  g.poster.resume();
  g.postTimer = setInterval(() => {
    if (game === g && g.posting) g.poster.tick();
  }, POST_MS);
}

function updateHud(view, g) {
  setText($("hud-score"), String(view.score));
  setText($("hud-time"), view.time.toFixed(1));
  setText($("hud-lives"), String(Math.max(0, view.lives)));
  setText($("hud-role"), roleName(view));
  updateRoundPanel(view, g);
  // The badge covers the live round; the round panel tells the same
  // story once the round pauses.
  $("friend-note").hidden = g.opponentPresent || view.phase === "over";
}

// roleName is this player's controls. The server rolls which seat pilots
// each round and sends the roll on the shared stream; our own seat is
// ours, so each client reads its own role off shared state.
function roleName(view) {
  if (!session || !view.pilot) return "";
  return view.pilot === session.seat ? "Pilot" : "Gunner";
}

// updateRoundPanel shows the two moments a round pauses for its players:
// the announcement that opens it, and the game-over screen that ends it.
// The final score and survival timer freeze when the round ends, so they
// are still correct while the screen is up.
function updateRoundPanel(view, g) {
  const paused = view.phase === "waiting" || view.phase === "over";
  if ($("round").hidden === paused) {
    $("round").hidden = !paused;
    // The panel spends the arena's height budget, so re-measure: the
    // arena gives up the height rather than the panel pushing the band
    // off the bottom of the screen.
    resize();
  }
  if (!paused) return;

  const role = roleName(view);
  if (view.phase === "waiting") {
    setText($("round-title"),
      role ? `Get ready — you're the ${role.toLowerCase()}` : "Get ready");
    $("round-stats").hidden = true;
    $("friend-wait").hidden = true;
    $("play-again").hidden = true;
    $("round-leave").hidden = true;
    return;
  }
  setText($("round-title"), "Game over");
  setText($("round-stats"),
    `Round ${view.round} · Score ${view.score} · Survived ${view.time.toFixed(1)}s`);
  $("round-stats").hidden = false;
  // A restart needs both players, so while the friend is gone the button
  // would only 409. Say what is actually happening instead, give the
  // button back the moment presence returns, and offer the one thing that
  // waiting on its own does not: the way back to the player's own room.
  $("friend-wait").hidden = g.opponentPresent;
  $("play-again").hidden = !g.opponentPresent;
  $("round-leave").hidden = g.opponentPresent;
}

function setText(el, text) {
  if (el.textContent !== text) el.textContent = text;
}

// applyPresence records the other seat's presence, which the stream
// reports on every flip while a round runs. A returning friend revives
// the poster a 409 stopped, so a player who kept holding a control
// resumes sending without re-pressing; the badge and the round panel
// follow from the flag on the next frame.
function applyPresence(present, g) {
  if (g.opponentPresent === present) return;
  g.opponentPresent = present;
  if (present) g.poster.revive();
}

// connectStream opens one streaming fetch and reads SSE frames until it
// ends. Because EventSource cannot set the player header or expose the
// status, the client reads the body itself and reconnects with backoff,
// or re-enters the lobby on 401/404.
async function connectStream() {
  const g = game;
  if (!g || g.stopped) return;

  g.controller = new AbortController();
  let res;
  try {
    res = await fetch(`${API}/lobby/${encodeURIComponent(g.lobbyId)}/events`, {
      headers: g.token ? { "X-Player-Token": g.token } : {},
      signal: g.controller.signal,
    });
  } catch (e) {
    if (g.stopped) return;
    return scheduleReconnect();
  }
  if (g.stopped) return;

  if (!res.ok) {
    if (res.status === 401 || res.status === 404) {
      stopGame();
      await openLobby(g.lobbyId);
      return;
    }
    if (res.status === 409) {
      // The room is waiting again: stop playing and poll until ready.
      stopGame();
      startPolling();
      return;
    }
    return scheduleReconnect();
  }

  // A new connection is a fresh timeline: reset the seq watermark and the
  // interpolation buffer, since a reclaimed lobby's sim restarts seq at 1.
  g.buffer = new Interp.SnapshotBuffer();
  g.connected = true;
  g.retryMs = 0;
  // The room was ready to connect, so both seats are present again; this
  // also revives a poster a 409 stopped during the gap.
  applyPresence(true, g);

  try {
    await readStream(res.body, g);
  } catch {
    // Network error or abort; a stopped game is handled below.
  }
  if (g.stopped) return;
  scheduleReconnect();
}

function scheduleReconnect() {
  const g = game;
  if (!g || g.stopped) return;
  g.connected = false;
  g.retryMs = g.retryMs ? Math.min(g.retryMs * 2, RECONNECT_MAX_MS) : RECONNECT_MIN_MS;
  g.timer = setTimeout(() => {
    if (game === g && !g.stopped) connectStream();
  }, g.retryMs);
}

// readStream parses SSE frames out of the response body and pushes each
// snapshot with its local arrival time.
async function readStream(body, g) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffered = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) return;
    buffered += decoder.decode(value, { stream: true });
    let end;
    while ((end = buffered.indexOf("\n\n")) >= 0) {
      const frame = buffered.slice(0, end);
      buffered = buffered.slice(end + 2);
      if (g.stopped) return;
      handleFrame(frame, g);
    }
  }
}

// handleFrame extracts one SSE frame's event name and JSON payload.
// Comments (the keepalive) carry no data and are ignored; a presence
// event updates the player's view of the other seat, everything else is
// a snapshot for the interpolation buffer.
function handleFrame(frame, g) {
  let event = "";
  const dataLines = [];
  for (let line of frame.split("\n")) {
    if (line.endsWith("\r")) line = line.slice(0, -1);
    if (line.startsWith(":")) continue;
    if (line.startsWith("event:")) event = line.slice(6).replace(/^ /, "");
    if (line.startsWith("data:")) dataLines.push(line.slice(5).replace(/^ /, ""));
  }
  if (dataLines.length === 0) return;
  let data;
  try {
    data = JSON.parse(dataLines.join("\n"));
  } catch {
    return;
  }
  if (event === "presence") {
    applyPresence(Boolean(data.present), g);
    return;
  }
  g.buffer.push(data, performance.now());
}

// --- rendering ---

// waitingView is the panel a player has while the room is not ready: their
// own share link, or — for a guest in somebody else's room — the notice
// that the room's owner is away.
function waitingView() {
  return session && session.seat === "guest" ? "guest" : "lobby";
}

// render fills in and shows that panel. The share box is always built from
// the player's own identity, never from the link the page was opened with:
// that link belongs to whoever owns the room, and is not the visitor's to
// hand out. The shared stream, not this panel, is what says the round is
// starting.
function render() {
  const view = waitingView();
  if (view === "lobby") {
    $("share-link").value = `${location.origin}/?r=${identity.lobbyId}`;
  }
  show(view);
}

function show(view) {
  for (const v of ["lobby", "guest", "error"]) $(v).hidden = v !== view;
  // The spinner runs only while a view is waiting on the other player:
  // the owner's room or somebody else's.
  $("spinner").hidden = view !== "lobby";
  $("guest-spinner").hidden = view !== "guest";
}

function showError(message) {
  clearInterval(pollTimer);
  stopGame();
  session = null;
  $("error-message").textContent = message;
  show("error");
}

function errorMessage(e) {
  if (e.status === 404) return "This link doesn't point to a lobby.";
  if (e.status === 409) return "This lobby is already full.";
  return "Something went wrong. Check your connection and reload.";
}

// --- share affordance ---

function shareLink() {
  return $("share-link").value;
}

if (navigator.share) {
  $("share-button").hidden = false;
}

$("share-button").addEventListener("click", async () => {
  try {
    await navigator.share({
      title: "Two Idiots, One Ship",
      url: shareLink(),
    });
  } catch {
    // The user dismissed the share sheet; nothing to do.
  }
});

$("copy").addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(shareLink());
    $("copy").textContent = "Copied!";
  } catch {
    // Clipboard unavailable: the link is selectable in the input.
  }
});

$("retry").addEventListener("click", () => location.reload());

// --- leaving ---

// leaveRoom goes to the player's own room, clearing any other player's
// ?r= from the address bar on the way. It is the way out of someone
// else's link, of an error (reloading would keep the bad link in the URL
// and land on the same error) and of a round whose friend is gone. The
// pagehide beacon releases the seat held here as the navigation starts.
function leaveRoom() {
  location.href = "/";
}

$("guest-leave").addEventListener("click", leaveRoom);
$("round-leave").addEventListener("click", leaveRoom);
$("home").addEventListener("click", leaveRoom);

// --- round flow ---

// play-again asks the server for a fresh round. A 409 means the other
// player already restarted (or play is still under way); the stream shows
// the round that actually exists either way.
$("play-again").addEventListener("click", async () => {
  if (!game) return;
  try {
    await fetchJSON(`${API}/lobby/${encodeURIComponent(game.lobbyId)}/restart`, {
      method: "POST",
      headers: game.token ? { "X-Player-Token": game.token } : {},
    });
  } catch {
    // No action: the snapshot stream is the authority on the round.
  }
});

// --- goodbye ---

// A closing tab says goodbye at once, so the other player is not left
// waiting out the presence TTL. Backgrounding or locking a device does
// not fire pagehide, and the open stream's own presence covers both.
addEventListener("pagehide", () => {
  if (!session || !session.token || !navigator.sendBeacon) return;
  const body = new Blob([JSON.stringify({ token: session.token })], {
    type: "application/json",
  });
  navigator.sendBeacon(
    `${API}/lobby/${encodeURIComponent(session.lobbyId)}/leave`, body);
});

// --- controls ---
//
// The band is wired once. Which role is live is the round's roll, so the
// handlers are permanent and ask for the current role; setRole() below is
// what follows the roll.
const controls = Controls.attach({
  band: $("controls"),
  stick: $("stick"),
  aim: $("aim"),
  base: $("pad-base"),
  knob: $("pad-knob"),
  rest: $("rest"),
  restLabel: $("rest-label"),
  canvas,
  onIntent: (payload, urgent) => {
    if (game && game.poster) game.poster.set(payload, urgent);
  },
});

start();
