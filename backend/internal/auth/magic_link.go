// backend/internal/auth/magic_link.go
package auth

import (
	"encoding/json"
	"net/http"
)

type magicLinkRequest struct {
	Email string `json:"email"`
}

// MagicLink handles POST /api/auth/magic-link.
// Always returns 200 — never reveals whether the email is registered.
func (h *Handler) MagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	user, err := h.db.GetUserByEmail(r.Context(), req.Email)
	if err != nil || !user.IsActive {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.sendMagicLinkToUser(r.Context(), user, "login"); err != nil {
		if h.log != nil {
			h.log.Error("magic link send failed", "error", err, "user_id", user.ID)
		}
		// Still 200 — no enumeration
	}

	w.WriteHeader(http.StatusOK)
}
