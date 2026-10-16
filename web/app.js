const API = "/api";
const POLL_MS = 2000;

const $ = (id) => document.getElementById(id);
const canvas = $("arena");
const ctx = canvas.getContext("2d");

function resize() {
  canvas.width = canvas.clientWidth * devicePixelRatio;
  canvas.height = canvas.clientHeight * devicePixelRatio;
}

addEventListener("resize", resize);
resize();

// --- room helpers ---

function roomId() {
  return new URLSearchParams(location.search).get("r");
}

function tokenKey(id) {
  return `asteroids-room:${id}`;
}

function loadToken(id) {
  try {
    return JSON.parse(localStorage.getItem(tokenKey(id)))?.token || "";
  } catch {
    return "";
  }
}

function saveToken(id, data) {
  try {
    localStorage.setItem(tokenKey(id), JSON.stringify(data));
  } catch {
    // storage unavailable: reconnect just won't survive a refresh
  }
}

function show(view) {
  for (const v of ["home", "room", "error"]) $(v).hidden = v !== view;
}

function showError(message) {
  clearInterval(pollTimer);
  $("error-message").textContent = message;
  show("error");
}

async function postJSON(url, body) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body || {}),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(data.error || `request failed (${res.status})`);
    err.status = res.status;
    throw err;
  }
  return data;
}

// --- room flow ---

let pollTimer = null;

async function createRoom() {
  try {
    const res = await postJSON(`${API}/rooms`, {});
    location.search = `?r=${encodeURIComponent(res.id)}`;
  } catch {
    showError("Could not create a room. Check your connection and try again.");
  }
}

async function joinRoom(id) {
  let res;
  try {
    res = await postJSON(`${API}/rooms/${encodeURIComponent(id)}/join`, {
      token: loadToken(id),
    });
  } catch (e) {
    if (e.status === 404) {
      showError("Room not found. It may have expired — ask for a fresh link.");
    } else if (e.status === 409) {
      showError("This room is full. Only two players can join.");
    } else {
      showError("Could not join the room. Check your connection and reload.");
    }
    return;
  }
  saveToken(id, { role: res.role, token: res.token });
  enterRoom(id, res.role, res.token);
}

function enterRoom(id, role, token) {
  show("room");
  $("role").textContent = role;
  $("share-link").value = `${location.origin}${location.pathname}?r=${id}`;
  poll(id, token);
  clearInterval(pollTimer);
  pollTimer = setInterval(() => poll(id, token), POLL_MS);
}

async function poll(id, token) {
  let res;
  try {
    const http = await fetch(`${API}/rooms/${encodeURIComponent(id)}`, {
      headers: { "X-Player-Token": token },
    });
    if (!http.ok) {
      const err = new Error(`status ${http.status}`);
      err.status = http.status;
      throw err;
    }
    res = await http.json();
  } catch (e) {
    if (e.status === 404) {
      showError("The room has expired. Ask for a fresh link.");
    }
    // transient network errors: keep polling
    return;
  }

  if (res.state === "ready") {
    $("state").textContent = "Both players are in. Game starting soon!";
  } else {
    $("state").textContent = "Waiting for the other player to open the link…";
  }
  const other = res.role === "pilot" ? "gunner" : "pilot";
  const otherHere = res.role === "pilot" ? res.gunner : res.pilot;
  if (res.state === "ready") {
    $("partner").textContent = otherHere
      ? `Your ${other} is here.`
      : `Your ${other} is disconnected — waiting for them to come back.`;
  } else {
    $("partner").textContent = "";
  }
}

$("create").addEventListener("click", createRoom);
$("copy").addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText($("share-link").value);
    $("copy").textContent = "Copied!";
  } catch {
    // clipboard unavailable: the link is selectable in the input
  }
});
$("home-again").addEventListener("click", () => {
  location.search = "";
});

const id = roomId();
if (id) {
  joinRoom(id);
} else {
  show("home");
}
