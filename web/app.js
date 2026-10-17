const API = "/api";
const POLL_MS = 2000;
const IDENTITY_KEY = "asteroids-identity";

const $ = (id) => document.getElementById(id);

// The arena canvas is the eventual game surface; size it now so the
// renderer task can draw into it.
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

// session is the lobby this page is looking at.
let session = null;
let pollTimer = null;

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

// openLobby enters (or re-enters) a lobby and starts polling it. It is
// used for the first visit and to recover after an instance is reclaimed
// or a guest seat is lost.
async function openLobby(lobbyId) {
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
    owner: data.seat === "owner",
    token: data.seat === "owner" ? me.token : data.token || credential,
    path: data.path,
  };
  render(data);

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

start();
