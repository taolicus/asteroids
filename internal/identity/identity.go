// Package identity issues and verifies stateless player identities.
//
// A lobby identity is a public, random lobby ID plus a secret owner
// credential. The credential is HMAC-signed with a key from
// configuration, so the server can verify it without storing anything:
// identities survive process restarts and room reclamation. The lobby
// ID goes into shared links; the credential never leaves the owner's
// browser. See tasks/closed/player-owned-lobby-rooms.md.
package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const (
	// idBytes is the entropy in a lobby ID; base64url without padding
	// yields IDLength characters.
	idBytes = 9
	// IDLength is the length of a lobby ID.
	IDLength = 12
	// sigBytes truncates the HMAC to 128 bits. Tokens stay short and
	// forgery resistance stays far beyond any practical attack.
	sigBytes = 16
)

// ErrInvalid is returned for any credential that does not verify.
var ErrInvalid = errors.New("invalid identity token")

// Identity is a freshly issued lobby identity.
type Identity struct {
	// LobbyID is public: it identifies the lobby and appears in links.
	LobbyID string
	// Token is the secret owner credential, stored only in the owner's
	// browser and sent to claim the owner seat.
	Token string
}

// Issuer signs and verifies credentials with one configured key.
type Issuer struct{ key []byte }

// NewIssuer returns an issuer using key as its HMAC key. The key must be
// stable across restarts or existing credentials stop verifying.
func NewIssuer(key []byte) *Issuer { return &Issuer{key: key} }

// Issue creates a new identity with a fresh random lobby ID.
func (i *Issuer) Issue() (Identity, error) {
	id, err := randomID()
	if err != nil {
		return Identity{}, err
	}
	return Identity{LobbyID: id, Token: id + "." + i.sign(id)}, nil
}

// Verify returns the lobby ID of a valid owner credential.
func (i *Issuer) Verify(token string) (string, error) {
	id, sig, ok := strings.Cut(token, ".")
	if !ok || !ValidID(id) || sig == "" {
		return "", ErrInvalid
	}
	if !hmac.Equal([]byte(sig), []byte(i.sign(id))) {
		return "", ErrInvalid
	}
	return id, nil
}

func (i *Issuer) sign(id string) string {
	mac := hmac.New(sha256.New, i.key)
	mac.Write([]byte("owner|" + id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:sigBytes])
}

// ValidID reports whether id has the shape of a lobby ID. Links carrying
// anything else are rejected instead of silently creating a dead lobby.
func ValidID(id string) bool {
	if len(id) != IDLength {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func randomID() (string, error) {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
