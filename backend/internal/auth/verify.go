// backend/internal/auth/verify.go
package auth

import (
	"encoding/json"
	"net/http"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

type verifyRequest struct {
	Token string `json:"token"`
}

// Verify handles POST /api/auth/verify.
// Consumes a magic link token (one-time use) and creates a session.
// For email_change tokens, swaps the email and invalidates all sessions.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	tokenHash := HashToken(req.Token)
	token, err := h.db.ConsumeMagicLinkTokenAtomic(r.Context(), tokenHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "Token is invalid, expired, or already used")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), token.UserID)
	if err != nil {
		if h.log != nil {
			h.log.Error("verify: get user failed", "error", err)
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal error")
		return
	}
	if !user.IsActive {
		writeError(w, http.StatusUnauthorized, "account_inactive", "Account is not active")
		return
	}

	switch token.Purpose {
	case "login":
		h.createSessionAndRespond(w, r, user)

	case "email_change":
		oldEmail := user.Email

		updated, err := h.db.ConfirmEmailChange(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Email change failed")
			return
		}

		if err := h.db.DeleteAllUserSessions(r.Context(), user.ID); err != nil {
			if h.log != nil {
				h.log.Error("verify: delete sessions failed", "error", err)
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "Session invalidation failed")
			return
		}

		if err := h.email.SendEmailChangedNotification(r.Context(), oldEmail, EmailChangedNotificationData{
			Username:       updated.Username,
			NewEmailMasked: maskEmail(updated.Email),
			FrontendURL:    h.cfg.FrontendURL,
		}); err != nil && h.log != nil {
			h.log.Error("verify: email changed notification", "error", err)
		}

		h.createSessionAndRespond(w, r, updated)

	default:
		if h.log != nil {
			h.log.Error("verify: unknown token purpose", "purpose", token.Purpose)
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Unknown token purpose")
	}
}

func (h *Handler) createSessionAndRespond(w http.ResponseWriter, r *http.Request, user db.User) {
	rawToken, err := GenerateRawToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Session creation failed")
		return
	}
	sessionHash := HashToken(rawToken)
	expiresAt := time.Now().UTC().Add(h.cfg.SessionTTL)

	if _, err := h.db.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: sessionHash,
		ExpiresAt: expiresAt,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Session creation failed")
		return
	}

	setSessionCookie(w, rawToken, h.cfg.SessionTTL)
	writeJSON(w, http.StatusOK, map[string]string{"user_id": user.ID.String()})
}
