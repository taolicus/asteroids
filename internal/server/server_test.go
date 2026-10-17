package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/taolicus/asteroids/internal/game"
	"github.com/taolicus/asteroids/internal/identity"
	"github.com/taolicus/asteroids/internal/rooms"
)

const testKey = "server-test-key"

// testServer bundles the HTTP server with the managers it was built
// from, so tests can inspect the sim layer.
type testServer struct {
	*httptest.Server
	sessions *game.Sessions
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	return newTestServerWithKey(t, testKey)
}

func newTestServerWithKey(t *testing.T, key string) *testServer {
	t.Helper()
	sessions := game.NewSessions(rooms.IdleGrace)
	ts := httptest.NewServer(Handler(rooms.NewManager(), identity.NewIssuer([]byte(key)), sessions))
	t.Cleanup(func() {
		ts.Close()
		sessions.StopAll()
	})
	return &testServer{Server: ts, sessions: sessions}
}

func doJSON(t *testing.T, method, url, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set(playerTokenHeader, token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	var data map[string]any
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatalf("decode %s %s: %v", method, url, err)
	}
	return res, data
}

// issueIdentity creates a fresh identity and returns its lobby ID and
// owner credential.
func issueIdentity(t *testing.T, ts *testServer) (lobbyID, token string) {
	t.Helper()
	res, data := doJSON(t, http.MethodPost, ts.URL+"/api/identities", "")
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("identities status = %d, want 201", res.StatusCode)
	}
	lobbyID, _ = data["lobbyId"].(string)
	token, _ = data["token"].(string)
	if !identity.ValidID(lobbyID) {
		t.Fatalf("lobby ID %q is not valid", lobbyID)
	}
	if token == "" {
		t.Fatal("identity response missing token")
	}
	return lobbyID, token
}

func enterLobby(t *testing.T, ts *testServer, id, token string) (*http.Response, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/lobby/%s", ts.URL, id), token)
}

func lobbyStatus(t *testing.T, ts *testServer, id, token string) (*http.Response, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/lobby/%s", ts.URL, id), token)
}

// postIntent sends a raw intent body and returns the response with its
// body still readable by the caller.
func postIntent(t *testing.T, ts *testServer, id, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/lobby/%s/intent", ts.URL, id), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set(playerTokenHeader, token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

// readyLobby returns a lobby with both seats filled, plus the owner and
// guest tokens and the owner's lobby ID.
func readyLobby(t *testing.T, ts *testServer) (id, ownerToken, guestToken string) {
	t.Helper()
	id, ownerToken = issueIdentity(t, ts)
	enterLobby(t, ts, id, ownerToken)
	_, data := enterLobby(t, ts, id, "")
	guestToken, _ = data["token"].(string)
	if guestToken == "" {
		t.Fatalf("guest enter did not return a token: %v", data)
	}
	return id, ownerToken, guestToken
}

// postRestart asks the room for a fresh round with the given token.
func postRestart(t *testing.T, ts *testServer, id, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/lobby/%s/restart", ts.URL, id), nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set(playerTokenHeader, token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

// tokenFor returns the credential of whichever seat holds role this
// round. Roles are rolled, so a test must ask the sim rather than assume
// the owner pilots.
func tokenFor(t *testing.T, sim *game.Sim, role game.Role, ownerToken, guestToken string) string {
	t.Helper()
	ownerRole, err := sim.RoleOf(game.PlayerOne)
	if err != nil {
		t.Fatalf("RoleOf(owner) = %v", err)
	}
	guestRole, err := sim.RoleOf(game.PlayerTwo)
	if err != nil {
		t.Fatalf("RoleOf(guest) = %v", err)
	}
	if ownerRole == guestRole {
		t.Fatalf("both seats hold %q", ownerRole)
	}
	if ownerRole == role {
		return ownerToken
	}
	if guestRole == role {
		return guestToken
	}
	t.Fatalf("no seat holds %q (owner=%q guest=%q)", role, ownerRole, guestRole)
	return ""
}

// waitForPlaySim drains a sim mailbox until the round's field goes live.
func waitForPlaySim(t *testing.T, ch <-chan game.Snapshot) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case snap, ok := <-ch:
			if !ok {
				t.Fatal("mailbox closed before play began")
			}
			if snap.Phase == game.PhasePlaying {
				return
			}
		case <-deadline:
			t.Fatal("round never began playing")
		}
	}
}

func TestIdentityEndpoint(t *testing.T) {
	ts := newTestServer(t)
	id1, tok1 := issueIdentity(t, ts)
	id2, _ := issueIdentity(t, ts)
	if id1 == id2 {
		t.Fatal("two identities share a lobby ID")
	}
	if tok1 == "" {
		t.Fatal("missing credential")
	}
}

func TestOwnerAndGuestFlow(t *testing.T) {
	ts := newTestServer(t)
	id, token := issueIdentity(t, ts)

	// The owner enters their own lobby.
	res, data := enterLobby(t, ts, id, token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("owner enter = %d, %v", res.StatusCode, data)
	}
	if data["seat"] != "owner" || data["state"] != "waiting" ||
		data["owner"] != true || data["guest"] != false {
		t.Fatalf("owner enter body = %v", data)
	}
	if want := "/?r=" + id; data["path"] != want {
		t.Fatalf("path = %v, want %q", data["path"], want)
	}
	if data["token"] != nil {
		t.Fatalf("owner was issued a reconnect token: %v", data["token"])
	}

	// A guest opens the link with no credential and gets the guest seat.
	res, data = enterLobby(t, ts, id, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("guest enter = %d, %v", res.StatusCode, data)
	}
	guestToken, _ := data["token"].(string)
	if data["seat"] != "guest" || data["state"] != "ready" || guestToken == "" {
		t.Fatalf("guest enter body = %v", data)
	}

	// The owner polls and sees the guest.
	res, data = lobbyStatus(t, ts, id, token)
	if res.StatusCode != http.StatusOK || data["seat"] != "owner" || data["state"] != "ready" ||
		data["owner"] != true || data["guest"] != true {
		t.Fatalf("owner status = %d, %v", res.StatusCode, data)
	}
	// The guest polls with their own token.
	res, data = lobbyStatus(t, ts, id, guestToken)
	if res.StatusCode != http.StatusOK || data["seat"] != "guest" || data["state"] != "ready" {
		t.Fatalf("guest status = %d, %v", res.StatusCode, data)
	}
}

func TestGuestWaitsForAbsentOwner(t *testing.T) {
	ts := newTestServer(t)
	id, _ := issueIdentity(t, ts)

	// Only the guest is here; the owner's seat stays reserved.
	_, data := enterLobby(t, ts, id, "")
	if data["seat"] != "guest" || data["state"] != "waiting" || data["owner"] != false {
		t.Fatalf("guest-first enter = %v", data)
	}
}

func TestSecondGuestSeesFull(t *testing.T) {
	ts := newTestServer(t)
	id, _ := issueIdentity(t, ts)
	enterLobby(t, ts, id, "")
	enterLobby(t, ts, id, "")

	res, data := enterLobby(t, ts, id, "")
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("second guest = %d (%v), want 409", res.StatusCode, data)
	}
}

func TestForeignCredentialIsTreatedAsGuest(t *testing.T) {
	ts := newTestServer(t)
	id, _ := issueIdentity(t, ts)
	_, foreignToken := issueIdentity(t, ts) // belongs to another lobby

	res, data := enterLobby(t, ts, id, foreignToken)
	if res.StatusCode != http.StatusOK || data["seat"] != "guest" {
		t.Fatalf("foreign credential enter = %d, %v; want guest", res.StatusCode, data)
	}
}

func TestLobbyIDValidation(t *testing.T) {
	ts := newTestServer(t)
	for _, bad := range []string{"short", "not-base64url!", "toolongtobeanid"} {
		res, _ := enterLobby(t, ts, bad, "")
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("enter %q = %d, want 404", bad, res.StatusCode)
		}
		res, _ = lobbyStatus(t, ts, bad, "whatever")
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("status %q = %d, want 404", bad, res.StatusCode)
		}
	}
}

func TestStatusAuthentication(t *testing.T) {
	ts := newTestServer(t)
	id, token := issueIdentity(t, ts)
	enterLobby(t, ts, id, token)
	_, guestData := enterLobby(t, ts, id, "")
	guestToken, _ := guestData["token"].(string)

	res, _ := lobbyStatus(t, ts, id, "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", res.StatusCode)
	}
	res, _ = lobbyStatus(t, ts, id, "wrong-token")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", res.StatusCode)
	}
	// A valid-shaped but never-used lobby ID is unknown, not an error.
	res, _ = lobbyStatus(t, ts, "zzzzzzzzzzzz", guestToken)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown lobby = %d, want 404", res.StatusCode)
	}
}

func TestCredentialSurvivesRestartWithSameKey(t *testing.T) {
	first := newTestServerWithKey(t, testKey)
	id, token := issueIdentity(t, first)

	// A restart is a new process: fresh manager and issuer, same key.
	second := newTestServerWithKey(t, testKey)
	res, data := enterLobby(t, second, id, token)
	if res.StatusCode != http.StatusOK || data["seat"] != "owner" {
		t.Fatalf("after restart = %d, %v; want owner seat", res.StatusCode, data)
	}
	if want := "/?r=" + id; data["path"] != want {
		t.Fatalf("link changed across restart: %v, want %q", data["path"], want)
	}
}

func TestCredentialFailsWithDifferentKey(t *testing.T) {
	first := newTestServerWithKey(t, testKey)
	id, token := issueIdentity(t, first)

	other := newTestServerWithKey(t, "a-different-key")
	res, data := enterLobby(t, other, id, token)
	if res.StatusCode != http.StatusOK || data["seat"] != "guest" {
		t.Fatalf("different key = %d, %v; want guest fallback", res.StatusCode, data)
	}
}

// TestFirstIntentStartsTheSim checks the room's loop starts lazily on the
// first submitted intent, like the first stream subscription.
func TestFirstIntentStartsTheSim(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, _ := readyLobby(t, ts)

	res := postIntent(t, ts, id, ownerToken, `{"seq":1,"thrust":true}`)
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusBadRequest {
		t.Fatalf("first intent = %d, want 200 or 400", res.StatusCode)
	}
	if ts.sessions.Get(id) == nil {
		t.Fatal("accepting an intent did not start a sim")
	}
}

func TestPilotIntentMovesTheShip(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	// Roles are rolled each round, so find the seat that is flying.
	sim := ts.sessions.Start(id)
	pilotToken := tokenFor(t, sim, game.RolePilot, ownerToken, guestToken)

	ch, cancel := sim.Subscribe()
	defer cancel()
	waitForPlaySim(t, ch)

	// Hold thrust, as a client posting a fresh intent would, and watch
	// the shared ship gain velocity.
	deadline := time.After(2 * time.Second)
	for seq := uint64(1); ; seq++ {
		res := postIntent(t, ts, id, pilotToken,
			fmt.Sprintf(`{"seq":%d,"thrust":true}`, seq))
		if res.StatusCode != http.StatusOK {
			t.Fatalf("pilot intent = %d, want 200", res.StatusCode)
		}
		select {
		case snap := <-ch:
			if snap.Ship.Vel.X != 0 || snap.Ship.Vel.Y != 0 {
				return // the pilot's thrust reached the simulation
			}
		case <-deadline:
			t.Fatal("ship never accelerated after a thrust intent")
		}
	}
}

// TestPilotSteerReachesTheSim follows the whole path the stick direction
// travels: a pilot's post, decoded by seat, turned into a heading by the
// running sim, and carried back out on the snapshots every client reads.
func TestPilotSteerReachesTheSim(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	sim := ts.sessions.Start(id)
	pilotToken := tokenFor(t, sim, game.RolePilot, ownerToken, guestToken)

	ch, cancel := sim.Subscribe()
	defer cancel()
	waitForPlaySim(t, ch)

	const steer = -2.5 // radians, well off the spawn heading of 0
	deadline := time.After(2 * time.Second)
	for seq := uint64(1); ; seq++ {
		res := postIntent(t, ts, id, pilotToken,
			fmt.Sprintf(`{"seq":%d,"steer":%v}`, seq, steer))
		if res.StatusCode != http.StatusOK {
			t.Fatalf("pilot intent = %d, want 200", res.StatusCode)
		}
		select {
		case snap := <-ch:
			if math.Abs(snap.Ship.Heading-steer) < 1e-6 {
				return // the ship turned to the posted direction
			}
		case <-deadline:
			t.Fatal("the pilot's steer never turned the ship")
		}
	}
}

func TestGunnerSeatUsesTheGunnerShape(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	sim := ts.sessions.Start(id)
	gunnerToken := tokenFor(t, sim, game.RoleGunner, ownerToken, guestToken)

	res := postIntent(t, ts, id, gunnerToken, `{"seq":1,"fire":true}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("gunner intent = %d, want 200", res.StatusCode)
	}
	// The independent turret is part of the gunner shape.
	res = postIntent(t, ts, id, gunnerToken, `{"seq":2,"fire":true,"aim":-1.5}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("gunner sending aim = %d, want 200", res.StatusCode)
	}
	// A non-numeric aim is still a decode error.
	res = postIntent(t, ts, id, gunnerToken, `{"seq":3,"fire":true,"aim":"left"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("gunner sending a text aim = %d, want 400", res.StatusCode)
	}
	// The gunner must not be able to send pilot fields.
	res = postIntent(t, ts, id, gunnerToken, `{"seq":4,"rotate":1}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("gunner sending rotate = %d, want 400", res.StatusCode)
	}
	res = postIntent(t, ts, id, gunnerToken, `{"seq":5,"steer":0}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("gunner sending steer = %d, want 400", res.StatusCode)
	}
}

// TestGunnerAimReachesTheSim follows the whole path the aim travels: a
// gunner's post, decoded by seat, stored by the round, and carried back
// out on the snapshots every client reads.
func TestGunnerAimReachesTheSim(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	sim := ts.sessions.Start(id)
	gunnerToken := tokenFor(t, sim, game.RoleGunner, ownerToken, guestToken)

	ch, cancel := sim.Subscribe()
	defer cancel()
	waitForPlaySim(t, ch)

	const aim = 1.25 // radians, a quarter turn off any default
	deadline := time.After(2 * time.Second)
	for seq := uint64(1); ; seq++ {
		res := postIntent(t, ts, id, gunnerToken,
			fmt.Sprintf(`{"seq":%d,"aim":%v}`, seq, aim))
		if res.StatusCode != http.StatusOK {
			t.Fatalf("gunner intent = %d, want 200", res.StatusCode)
		}
		select {
		case snap := <-ch:
			if snap.Turret == aim {
				return // the posted aim reached the round's state
			}
		case <-deadline:
			t.Fatal("the gunner's aim never reached the simulation")
		}
	}
}

func TestIntentRejections(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	// Pilot-shaped bodies need whichever seat is piloting this round.
	pilotToken := tokenFor(t, ts.sessions.Start(id), game.RolePilot, ownerToken, guestToken)

	// A lobby with only its owner present is not ready.
	loneID, loneOwner := issueIdentity(t, ts)
	enterLobby(t, ts, loneID, loneOwner)

	tests := []struct {
		name  string
		id    string
		token string
		body  string
		want  int
	}{
		{"no token", id, "", `{"seq":1,"thrust":true}`, http.StatusUnauthorized},
		{"unknown token", id, "not-a-credential", `{"seq":1,"thrust":true}`, http.StatusUnauthorized},
		{"unknown lobby", "zzzzzzzzzzzz", ownerToken, `{"seq":1,"thrust":true}`, http.StatusNotFound},
		{"malformed json", id, pilotToken, `{`, http.StatusBadRequest},
		{"empty body", id, pilotToken, ``, http.StatusBadRequest},
		{"wrong type", id, pilotToken, `{"seq":"one","thrust":true}`, http.StatusBadRequest},
		{"rotate out of range", id, pilotToken, `{"seq":1,"rotate":2}`, http.StatusBadRequest},
		{"steer not a number", id, pilotToken, `{"seq":1,"steer":"left"}`, http.StatusBadRequest},
		{"other role's field", id, pilotToken, `{"seq":1,"fire":true}`, http.StatusBadRequest},
		{"gunner field on the pilot", id, pilotToken, `{"seq":1,"aim":1}`, http.StatusBadRequest},
		{"lobby not ready", loneID, loneOwner, `{"seq":1,"thrust":true}`, http.StatusConflict},
		{"oversized body", id, pilotToken, `{"seq":1,"thrust":true,"pad":"` + strings.Repeat("x", 600) + `"}`, http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := postIntent(t, ts, tt.id, tt.token, tt.body)
			if res.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.want)
			}
		})
	}
}

func TestIntentCoalescesDuplicatesAndCapsRate(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	pilotToken := tokenFor(t, ts.sessions.Start(id), game.RolePilot, ownerToken, guestToken)

	// Re-posting the same sequence is coalesced, not an error.
	for i := 0; i < 2; i++ {
		res := postIntent(t, ts, id, pilotToken, `{"seq":5,"thrust":true}`)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("duplicate seq = %d, want 200", res.StatusCode)
		}
	}

	// The coarse cap is 60 intents/second per role; the 61st in the same
	// window is refused.
	limited := false
	for seq := 6; seq <= 70; seq++ {
		res := postIntent(t, ts, id, pilotToken,
			fmt.Sprintf(`{"seq":%d,"thrust":true}`, seq))
		if res.StatusCode == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("rate cap never engaged within one window")
	}
}

// TestRestartStatusCodes covers the replay endpoint: either player may
// restart, and a round that is already waiting needs no restart at all.
func TestRestartStatusCodes(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	loneID, loneOwner := issueIdentity(t, ts)
	enterLobby(t, ts, loneID, loneOwner)

	tests := []struct {
		name  string
		id    string
		token string
		want  int
	}{
		{"owner restarts", id, ownerToken, http.StatusOK},
		{"guest restarts", id, guestToken, http.StatusOK},
		{"no token", id, "", http.StatusUnauthorized},
		{"unknown token", id, "not-a-credential", http.StatusUnauthorized},
		{"unknown lobby", "zzzzzzzzzzzz", ownerToken, http.StatusNotFound},
		{"malformed id", "short", ownerToken, http.StatusNotFound},
		{"lobby not ready", loneID, loneOwner, http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := postRestart(t, ts, tt.id, tt.token)
			if res.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.want)
			}
		})
	}
}

// TestRestartIsRefusedWhilePlaying checks a live round cannot be reset
// from under the copilots: a replay only works once game over freezes the
// field.
func TestRestartIsRefusedWhilePlaying(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, _ := readyLobby(t, ts)

	res := openStream(t, ts, id, ownerToken)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("events status = %d, want 200", res.StatusCode)
	}
	lines, stop := lineScanner(res.Body)
	defer stop()
	waitForPhase(t, lines, string(game.PhasePlaying))

	if res := postRestart(t, ts, id, ownerToken); res.StatusCode != http.StatusConflict {
		t.Fatalf("restart while playing = %d, want 409", res.StatusCode)
	}
}
