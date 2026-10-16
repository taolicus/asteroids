package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taolicus/asteroids/internal/rooms"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(Handler(rooms.NewManager()))
	t.Cleanup(ts.Close)
	return ts
}

func postJSON(t *testing.T, url string, body any, token string) (*http.Response, map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(playerTokenHeader, token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var data map[string]any
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatalf("decode %s %s: %v", res.Request.Method, url, err)
	}
	return res, data
}

func getStatus(t *testing.T, url, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(playerTokenHeader, token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var data map[string]any
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	return res, data
}

func createRoom(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	res, data := postJSON(t, ts.URL+"/api/rooms", map[string]any{}, "")
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", res.StatusCode)
	}
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatalf("create response missing id: %v", data)
	}
	if want := "/?r=" + id; data["link"] != want {
		t.Fatalf("link = %v, want %q", data["link"], want)
	}
	return id
}

func join(t *testing.T, ts *httptest.Server, id, token string) (string, string) {
	t.Helper()
	res, data := postJSON(t, fmt.Sprintf("%s/api/rooms/%s/join", ts.URL, id),
		map[string]any{"token": token}, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("join status = %d, body %v, want 200", res.StatusCode, data)
	}
	role, _ := data["role"].(string)
	tok, _ := data["token"].(string)
	return role, tok
}

func TestCreateJoinStatusFlow(t *testing.T) {
	ts := newTestServer(t)
	id := createRoom(t, ts)

	role, pilotToken := join(t, ts, id, "")
	if role != "pilot" || pilotToken == "" {
		t.Fatalf("first join = %q, %q", role, pilotToken)
	}
	role, gunnerToken := join(t, ts, id, "")
	if role != "gunner" || gunnerToken == "" {
		t.Fatalf("second join = %q, %q", role, gunnerToken)
	}

	res, data := getStatus(t, fmt.Sprintf("%s/api/rooms/%s", ts.URL, id), pilotToken)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if data["role"] != "pilot" || data["state"] != "ready" ||
		data["pilot"] != true || data["gunner"] != true {
		t.Fatalf("status body = %v", data)
	}
}

func TestJoinErrors(t *testing.T) {
	ts := newTestServer(t)
	id := createRoom(t, ts)
	join(t, ts, id, "")
	join(t, ts, id, "")

	res, _ := postJSON(t, fmt.Sprintf("%s/api/rooms/%s/join", ts.URL, id),
		map[string]any{}, "")
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("third join = %d, want 409", res.StatusCode)
	}

	res, data := postJSON(t, ts.URL+"/api/rooms/doesnotexist/join",
		map[string]any{}, "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown room join = %d (body %v), want 404", res.StatusCode, data)
	}
}

func TestTokenReattachWhenFull(t *testing.T) {
	ts := newTestServer(t)
	id := createRoom(t, ts)
	_, pilotToken := join(t, ts, id, "")
	join(t, ts, id, "")

	role, token := join(t, ts, id, pilotToken)
	if role != "pilot" || token != pilotToken {
		t.Fatalf("reattach = %q/%q, want pilot/own token", role, token)
	}
}

func TestStatusAuth(t *testing.T) {
	ts := newTestServer(t)
	id := createRoom(t, ts)
	join(t, ts, id, "")
	_, gunnerToken := join(t, ts, id, "")

	res, _ := getStatus(t, fmt.Sprintf("%s/api/rooms/%s", ts.URL, id), "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", res.StatusCode)
	}
	res, _ = getStatus(t, fmt.Sprintf("%s/api/rooms/%s", ts.URL, id), "wrong")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", res.StatusCode)
	}
	res, _ = getStatus(t, ts.URL+"/api/rooms/missing", gunnerToken)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown room = %d, want 404", res.StatusCode)
	}
}

func TestJoinRejectsInvalidJSON(t *testing.T) {
	ts := newTestServer(t)
	id := createRoom(t, ts)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/rooms/%s/join", ts.URL, id),
		bytes.NewReader([]byte("{not json")))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid body = %d, want 400", res.StatusCode)
	}
}
