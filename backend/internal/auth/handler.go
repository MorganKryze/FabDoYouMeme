// backend/internal/auth/handler.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

type Handler struct {
	db    *db.Queries
	pool  *pgxpool.Pool
	cfg   *config.Config
	email EmailSender
	log   *slog.Logger
	clock clock.Clock
}

// New constructs an auth Handler. Pass clock.Real{} in production; tests can
// inject a *clock.Fake to control session-expiry / magic-link-expiry windows.
func New(pool *pgxpool.Pool, cfg *config.Config, email EmailSender, log *slog.Logger, clk clock.Clock) *Handler {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Handler{
		db:    db.New(pool),
		pool:  pool,
		cfg:   cfg,
		email: email,
		log:   log,
		clock: clk,
	}
}

// SessionLookupFn satisfies middleware.SessionLookupFn.
//
// The renewal cadence is bounded by cfg.SessionRenewInterval: we only write a
// new expires_at when it would extend the row by at least that interval. On a
// busy authenticated API this turns hundreds of UPDATE statements per user
// into at most one per interval, without needing a dedicated last_renewed_at
// column — the extension delta itself tells us how long it has been since the
// previous renewal.
func (h *Handler) SessionLookupFn(ctx context.Context, tokenHash string) (string, string, string, string, bool, error) {
	row, err := h.db.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return "", "", "", "", false, err
	}
	newExpiry := h.clock.Now().Add(h.cfg.SessionTTL)
	if h.cfg.SessionRenewInterval <= 0 || newExpiry.Sub(row.ExpiresAt) >= h.cfg.SessionRenewInterval {
		if _, err := h.db.RenewSession(ctx, db.RenewSessionParams{
			ID:        row.ID,
			ExpiresAt: newExpiry,
		}); err != nil && h.log != nil {
			h.log.WarnContext(ctx, "session renewal failed", "err", err)
		}
	}
	return row.UID.String(), row.Username, row.Email, row.Role, row.IsActive, nil
}

// sendMagicLinkToUser invalidates prior tokens of the same purpose,
// generates a new one, persists its hash, and emails the raw token.
func (h *Handler) sendMagicLinkToUser(ctx context.Context, user db.User, purpose string) error {
	if err := h.db.InvalidatePendingTokens(ctx, db.InvalidatePendingTokensParams{
		UserID:  user.ID,
		Purpose: purpose,
	}); err != nil && h.log != nil {
		h.log.WarnContext(ctx, "sendMagicLink: failed to invalidate prior tokens",
			"user_id", user.ID, "purpose", purpose, "error", err)
	}

	rawToken, err := GenerateRawToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	tokenHash := HashToken(rawToken)
	expiresAt := h.clock.Now().Add(h.cfg.MagicLinkTTL)

	if _, err := h.db.CreateMagicLinkToken(ctx, db.CreateMagicLinkTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Purpose:   purpose,
		ExpiresAt: expiresAt,
	}); err != nil {
		return fmt.Errorf("store token: %w", err)
	}

	magicURL := h.cfg.MagicLinkBaseURL + "/auth/verify?token=" + rawToken
	expiryMinutes := int(h.cfg.MagicLinkTTL.Minutes())

	if purpose == "email_change" {
		if user.PendingEmail == nil || *user.PendingEmail == "" {
			return fmt.Errorf("email_change requested but pending_email is not set for user %s", user.ID)
		}
		return h.email.SendMagicLinkEmailChange(ctx, *user.PendingEmail, EmailChangeData{
			Username:      user.Username,
			MagicLinkURL:  magicURL,
			FrontendURL:   h.cfg.FrontendURL,
			ExpiryMinutes: expiryMinutes,
		})
	}
	return h.email.SendMagicLinkLogin(ctx, user.Email, LoginEmailData{
		Username:      user.Username,
		MagicLinkURL:  magicURL,
		FrontendURL:   h.cfg.FrontendURL,
		ExpiryMinutes: expiryMinutes,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	body := map[string]string{"error": message, "code": code}
	if r != nil {
		if reqID := chiMiddleware.GetReqID(r.Context()); reqID != "" {
			body["request_id"] = reqID
		}
	}
	writeJSON(w, status, body)
}

func maskEmail(email string) string {
	at := strings.Index(email, "@")
	if at < 0 {
		return "***"
	}
	return "***" + email[at:]
}

// SetSeedAdminEmail overrides cfg.SeedAdminEmail — for use in tests only.
func (h *Handler) SetSeedAdminEmail(email string) {
	h.cfg.SeedAdminEmail = email
}
