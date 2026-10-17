package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestIssueVerifies(t *testing.T) {
	iss := NewIssuer([]byte("test-key"))

	id, err := iss.Issue()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(id.LobbyID) {
		t.Fatalf("lobby ID %q is not valid", id.LobbyID)
	}
	if !strings.HasPrefix(id.Token, id.LobbyID+".") {
		t.Fatalf("token %q does not carry its lobby ID", id.Token)
	}
	got, err := iss.Verify(id.Token)
	if err != nil {
		t.Fatalf("verify own token: %v", err)
	}
	if got != id.LobbyID {
		t.Fatalf("verify = %q, want %q", got, id.LobbyID)
	}
}

func TestIssueIsUnique(t *testing.T) {
	iss := NewIssuer([]byte("test-key"))
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id, err := iss.Issue()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id.LobbyID] {
			t.Fatalf("duplicate lobby ID %q", id.LobbyID)
		}
		seen[id.LobbyID] = true
	}
}

func TestVerifyRejectsTampered(t *testing.T) {
	iss := NewIssuer([]byte("test-key"))
	id, _ := iss.Issue()
	other, _ := iss.Issue()

	cases := map[string]string{
		"empty":            "",
		"no separator":     id.LobbyID,
		"empty signature":  id.LobbyID + ".",
		"empty id":         "." + strings.SplitN(id.Token, ".", 2)[1],
		"foreign id":       other.LobbyID + "." + strings.SplitN(id.Token, ".", 2)[1],
		"flipped one char": flipLast(id.Token),
		"garbage":          "not-a-token",
	}
	for name, token := range cases {
		if _, err := iss.Verify(token); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestVerifyRejectsForeignKey(t *testing.T) {
	id, err := NewIssuer([]byte("key-a")).Issue()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewIssuer([]byte("key-b")).Verify(id.Token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign key err = %v, want ErrInvalid", err)
	}
}

func TestValidID(t *testing.T) {
	valid := []string{"abcdefghijkl", "ABCDEFGHIJKL", "0123456789-_"}
	for _, id := range valid {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false, want true", id)
		}
	}
	invalid := []string{"", "short", "too-long-to-be-an-id", "has space!!", "has.dot.ok!!", "has/slash!!!", "has+plus!!!", "has=equals="}
	for _, id := range invalid {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true, want false", id)
		}
	}
}

// flipLast returns token with its final character changed, producing the
// smallest possible tamper.
func flipLast(token string) string {
	if token == "" {
		return "x"
	}
	last := token[len(token)-1]
	repl := byte('A')
	if last == 'A' {
		repl = 'B'
	}
	return token[:len(token)-1] + string(repl)
}
