const API = "/api";
const POLL_MS = 2000;
const IDENTITY_KEY = "asteroids-identity";
const RECONNECT_MIN_MS = 500;
const RECONNECT_MAX_MS = 5000;

const $ = (id) => document.getElementById(id);

// The arena canvas is the game surface; size it to its CSS box now and on
// every resize.
const canvas = $("arena");

function resize() {
  canvas.width = canvas.clientWidth * devicePixelRatio;
  canvas.height = canvas.clientHeight * devicePixelRatio;
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
    path: data.path,
  };
  render(data);

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
  render(data);
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
  $("round").hidden = true;
  resize();

  game = {
    lobbyId: session.lobbyId,
    token: session.token,
    buffer: new Interp.SnapshotBuffer(),
    renderer: Renderer.create(canvas),
    controller: null,
    raf: 0,
    stopped: false,
    connected: false,
    retryMs: 0,
    timer: 0,
  };
  game.raf = requestAnimationFrame(frame);
  connectStream();
}

function stopGame() {
  if (!game) return;
  game.stopped = true;
  cancelAnimationFrame(game.raf);
  clearTimeout(game.timer);
  if (game.controller) game.controller.abort();
  game = null;
  $("arena").hidden = true;
  $("hud").hidden = true;
  $("round").hidden = true;
}

function frame(now) {
  const g = game;
  if (!g || g.stopped) return;

  const view = g.buffer.sample(now);
  if (view) {
    g.renderer.render(view, now);
    updateHud(view);
  }
  g.raf = requestAnimationFrame(frame);
}

function updateHud(view) {
  setText($("hud-score"), String(view.score));
  setText($("hud-time"), view.time.toFixed(1));
  setText($("hud-lives"), String(Math.max(0, view.lives)));
  setText($("hud-role"), roleName(view));
  updateRoundPanel(view);
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
function updateRoundPanel(view) {
  const paused = view.phase === "waiting" || view.phase === "over";
  $("round").hidden = !paused;
  if (!paused) return;

  const role = roleName(view);
  if (view.phase === "waiting") {
    setText($("round-title"),
      role ? `Get ready — you're the ${role.toLowerCase()}` : "Get ready");
    $("round-stats").hidden = true;
    $("play-again").hidden = true;
    return;
  }
  setText($("round-title"), "Game over");
  setText($("round-stats"),
    `Round ${view.round} · Score ${view.score} · Survived ${view.time.toFixed(1)}s`);
  $("round-stats").hidden = false;
  $("play-again").hidden = false;
}

function setText(el, text) {
  if (el.textContent !== text) el.textContent = text;
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

// handleFrame extracts the JSON payload of one SSE frame. Comments (the
// keepalive) carry no data and are ignored.
function handleFrame(frame, g) {
  const dataLines = [];
  for (let line of frame.split("\n")) {
    if (line.endsWith("\r")) line = line.slice(0, -1);
    if (line.startsWith(":")) continue;
    if (line.startsWith("data:")) dataLines.push(line.slice(5).replace(/^ /, ""));
  }
  if (dataLines.length === 0) return;
  let snap;
  try {
    snap = JSON.parse(dataLines.join("\n"));
  } catch {
    return;
  }
  g.buffer.push(snap, performance.now());
}

// --- rendering ---

function render(data) {
  show("lobby");
  const owner = data.seat === "owner";
  $("seat").textContent = owner
    ? "You own this lobby."
    : "You're the guest in this lobby.";
  $("state").textContent = owner ? ownerState(data) : guestState(data);
  $("share").hidden = !owner;
  if (owner) {
    $("share-link").value = `${location.origin}${data.path}`;
  }
}

function ownerState(data) {
  return data.guest
    ? "Your copilot is here."
    : "Waiting for your copilot to open the link.";
}

function guestState(data) {
  return data.owner
    ? "The owner is here."
    : "You're waiting for the owner to show up.";
}

function show(view) {
  for (const v of ["lobby", "error"]) $(v).hidden = v !== view;
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
      text: "Be my copilot:",
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

// --- round flow ---

// play-again asks the server for a fresh round. A 409 means the copilot
// already restarted (or play is still under way); the stream shows the
// round that actually exists either way.
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

start();
