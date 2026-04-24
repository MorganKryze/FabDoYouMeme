// backend/internal/auth/verify.go
package auth

import (
	"encoding/json"
	"net/http"

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
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	tokenHash := HashToken(req.Token)

	// Look up token first to return a specific error code.
	rawToken, lookupErr := h.db.GetMagicLinkTokenByHash(r.Context(), tokenHash)
	if lookupErr != nil {
		writeError(w, r, http.StatusBadRequest, "token_not_found", "Token not found")
		return
	}
	if rawToken.ExpiresAt.Before(h.clock.Now().UTC()) {
		writeError(w, r, http.StatusBadRequest, "token_expired", "Token has expired")
		return
	}
	if rawToken.UsedAt.Valid {
		writeError(w, r, http.StatusBadRequest, "token_used", "Token has already been used")
		return
	}

	// Atomically mark used — prevents replay race.
	token, err := h.db.ConsumeMagicLinkTokenAtomic(r.Context(), tokenHash)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "token_used", "Token has already been used")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), token.UserID)
	if err != nil {
		if h.log != nil {
			h.log.Error("verify: get user failed", "error", err)
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal error")
		return
	}
	if !user.IsActive {
		writeError(w, r, http.StatusForbidden, "user_inactive", "Account is not active")
		return
	}

	switch token.Purpose {
	case "login":
		h.createSessionAndRespond(w, r, user)

	case "email_change":
		oldEmail := user.Email

		updated, err := h.db.ConfirmEmailChange(r.Context(), user.ID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Email change failed")
			return
		}

		if err := h.db.DeleteAllUserSessions(r.Context(), user.ID); err != nil {
			if h.log != nil {
				h.log.Error("verify: delete sessions failed", "error", err)
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Session invalidation failed")
			return
		}

		notifyLocale := updated.Locale
		if notifyLocale == "" {
			notifyLocale = h.cfg.DefaultLocale
		}
		if err := h.email.SendEmailChangedNotification(r.Context(), oldEmail, notifyLocale, EmailChangedNotificationData{
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
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unknown token purpose")
	}
}

func (h *Handler) createSessionAndRespond(w http.ResponseWriter, r *http.Request, user db.User) {
	rawToken, err := GenerateRawToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Session creation failed")
		return
	}
	sessionHash := HashToken(rawToken)
	expiresAt := h.clock.Now().UTC().Add(h.cfg.SessionTTL)

	if _, err := h.db.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: sessionHash,
		ExpiresAt: expiresAt,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Session creation failed")
		return
	}

	// Stamp users.last_login_at — drives the 90-day auto-promotion scan. A
	// failure here is non-fatal: the session is already minted and the worst
	// downside is one delayed scan cycle, which is better than failing login.
	if err := h.db.TouchUserLastLogin(r.Context(), user.ID); err != nil && h.log != nil {
		h.log.Error("verify: touch last_login_at failed", "error", err, "user_id", user.ID)
	}

	setSessionCookie(w, rawToken, h.cfg.SessionTTL, h.cfg.CookieDomain)
	writeJSON(w, http.StatusOK, map[string]string{"user_id": user.ID.String()})
}
