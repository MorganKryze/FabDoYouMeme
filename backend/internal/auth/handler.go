// backend/internal/auth/handler.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

// magicLinkAsyncSendTimeout bounds how long a detached background send can
// block on SMTP before the goroutine gives up. The OVH relay typically
// answers in well under 2s; 30s is a generous safety net for transient
// latency without leaking goroutines indefinitely on a dead upstream.
const magicLinkAsyncSendTimeout = 30 * time.Second

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
	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "en"
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
func (h *Handler) SessionLookupFn(ctx context.Context, tokenHash string) (string, string, string, string, string, bool, time.Time, error) {
	row, err := h.db.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return "", "", "", "", "", false, time.Time{}, err
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
	return row.UID.String(), row.Username, row.Email, row.Role, row.Locale, row.IsActive, row.UCreatedAt, nil
}

// prepareMagicLinkToken invalidates any prior tokens of the same purpose,
// generates a fresh one, and persists its hash. All DB writes complete before
// return, so the token is guaranteed to exist by the time the caller dispatches
// the email — no race against a user who clicks the link the instant it arrives.
func (h *Handler) prepareMagicLinkToken(ctx context.Context, user db.User, purpose string) (magicURL string, expiryMinutes int, err error) {
	if invErr := h.db.InvalidatePendingTokens(ctx, db.InvalidatePendingTokensParams{
		UserID:  user.ID,
		Purpose: purpose,
	}); invErr != nil && h.log != nil {
		h.log.WarnContext(ctx, "sendMagicLink: failed to invalidate prior tokens",
			"user_id", user.ID, "purpose", purpose, "error", invErr)
	}

	rawToken, err := GenerateRawToken()
	if err != nil {
		return "", 0, fmt.Errorf("generate token: %w", err)
	}
	tokenHash := HashToken(rawToken)
	expiresAt := h.clock.Now().Add(h.cfg.MagicLinkTTL)

	if _, err := h.db.CreateMagicLinkToken(ctx, db.CreateMagicLinkTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Purpose:   purpose,
		ExpiresAt: expiresAt,
	}); err != nil {
		return "", 0, fmt.Errorf("store token: %w", err)
	}

	return h.cfg.MagicLinkBaseURL + "/auth/verify?token=" + rawToken,
		int(h.cfg.MagicLinkTTL.Minutes()),
		nil
}

// deliverMagicLink renders the correct email template for the purpose and
// hands it to the SMTP sender. Runs inline with the provided context — the
// wrappers decide whether that context is the request's or a detached one.
func (h *Handler) deliverMagicLink(ctx context.Context, user db.User, purpose, magicURL string, expiryMinutes int) error {
	locale := user.Locale
	if locale == "" {
		locale = h.cfg.DefaultLocale
	}
	if purpose == "email_change" {
		if user.PendingEmail == nil || *user.PendingEmail == "" {
			return fmt.Errorf("email_change requested but pending_email is not set for user %s", user.ID)
		}
		return h.email.SendMagicLinkEmailChange(ctx, *user.PendingEmail, locale, EmailChangeData{
			Username:      user.Username,
			MagicLinkURL:  magicURL,
			FrontendURL:   h.cfg.FrontendURL,
			ExpiryMinutes: expiryMinutes,
		})
	}
	return h.email.SendMagicLinkLogin(ctx, user.Email, locale, LoginEmailData{
		Username:      user.Username,
		MagicLinkURL:  magicURL,
		FrontendURL:   h.cfg.FrontendURL,
		ExpiryMinutes: expiryMinutes,
	})
}

// sendMagicLinkToUser persists a fresh token and sends the email synchronously.
// Used by the register, email-change, and bootstrap paths, all of which need
// to observe the send outcome (surface `smtp_failure` to the user, or fail
// server startup).
func (h *Handler) sendMagicLinkToUser(ctx context.Context, user db.User, purpose string) error {
	magicURL, expiryMinutes, err := h.prepareMagicLinkToken(ctx, user, purpose)
	if err != nil {
		return err
	}
	return h.deliverMagicLink(ctx, user, purpose, magicURL, expiryMinutes)
}

// sendMagicLinkToUserAsync persists the token synchronously then dispatches
// the SMTP send in a detached goroutine. The HTTP handler returns as soon as
// the token row is in Postgres; the ~1–2s SMTP round-trip no longer sits on
// the critical path. Used only by the high-frequency login endpoint, which
// already returns 200 regardless of send outcome for enumeration protection.
func (h *Handler) sendMagicLinkToUserAsync(ctx context.Context, user db.User, purpose string) error {
	magicURL, expiryMinutes, err := h.prepareMagicLinkToken(ctx, user, purpose)
	if err != nil {
		return err
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), magicLinkAsyncSendTimeout)
		defer cancel()
		if sendErr := h.deliverMagicLink(bgCtx, user, purpose, magicURL, expiryMinutes); sendErr != nil && h.log != nil {
			h.log.Error("magic link async send failed",
				"user_id", user.ID, "purpose", purpose, "error", sendErr)
		}
	}()
	return nil
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
