// backend/internal/auth/tokens.go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// GenerateRawToken returns a random 32-byte hex-encoded token.
func GenerateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of raw. Only the hash is stored in DB.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// setSessionCookie writes the session cookie with Lax SameSite policy.
// Lax (not Strict) is required so the cookie is sent on same-site top-level
// navigations with a null initiator — F5, address-bar reloads, external link
// clicks back into the app — without which the user appears logged-out on
// every refresh even though the cookie is still in the jar. Lax still blocks
// cookies on cross-site POSTs (the actual CSRF vector), so this is the right
// tradeoff for a session-cookie magic-link flow.
func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, domain string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, domain string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
