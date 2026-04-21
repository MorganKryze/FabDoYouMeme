// backend/internal/auth/session.go
package auth

import (
	"net/http"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// Logout handles POST /api/auth/logout.
// Deletes the session row and clears the session cookie.
// Intentionally idempotent: if the caller is not authenticated (no cookie or
// session already expired), we still return 200 and clear the cookie. This
// prevents information leakage and simplifies client-side logout flows.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil && cookie.Value != "" {
		hash := HashToken(cookie.Value)
		if err := h.db.DeleteSession(r.Context(), hash); err != nil && h.log != nil {
			h.log.Warn("logout: failed to delete session", "error", err)
		}
	}
	clearSessionCookie(w, h.cfg.CookieDomain)
	w.WriteHeader(http.StatusOK)
}

// Me handles GET /api/auth/me.
// Returns the current user's identity fields.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         u.UserID,
		"username":   u.Username,
		"email":      u.Email,
		"role":       u.Role,
		"locale":     u.Locale,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
	})
}
