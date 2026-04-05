// backend/internal/auth/session.go
package auth

import (
	"net/http"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// Logout handles POST /api/auth/logout.
// Deletes the session row and clears the session cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil && cookie.Value != "" {
		hash := HashToken(cookie.Value)
		_ = h.db.DeleteSession(r.Context(), hash)
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusOK)
}

// Me handles GET /api/auth/me.
// Returns the current user's identity fields.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":       u.UserID,
		"username": u.Username,
		"email":    u.Email,
		"role":     u.Role,
	})
}
