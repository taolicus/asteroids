package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taolicus/asteroids/internal/identity"
	"github.com/taolicus/asteroids/internal/rooms"
)

const testKey = "server-test-key"

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWithKey(t, testKey)
}

func newTestServerWithKey(t *testing.T, key string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(Handler(rooms.NewManager(), identity.NewIssuer([]byte(key))))
	t.Cleanup(ts.Close)
	return ts
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
func issueIdentity(t *testing.T, ts *httptest.Server) (lobbyID, token string) {
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

func enterLobby(t *testing.T, ts *httptest.Server, id, token string) (*http.Response, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/lobby/%s", ts.URL, id), token)
}

func lobbyStatus(t *testing.T, ts *httptest.Server, id, token string) (*http.Response, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/lobby/%s", ts.URL, id), token)
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
