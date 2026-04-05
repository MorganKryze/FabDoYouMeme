// backend/internal/auth/bootstrap.go
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// SeedAdmin creates the first admin user if SEED_ADMIN_EMAIL is set and no user
// with that email exists yet. Idempotent: safe to call on every startup.
func (h *Handler) SeedAdmin(ctx context.Context) error {
	if h.cfg.SeedAdminEmail == "" {
		return nil
	}

	// Check if the user already exists (idempotency)
	existing, err := h.db.GetUserByEmail(ctx, h.cfg.SeedAdminEmail)
	if err == nil {
		// User exists — do not modify; log if not admin
		if existing.Role != "admin" && h.log != nil {
			h.log.Warn("SEED_ADMIN_EMAIL is set but the account is not admin role; skipping bootstrap",
				"email", h.cfg.SeedAdminEmail)
		}
		return nil
	}

	// Create admin user with legitimate-interest consent_at (Art. 6(1)(f))
	consentAt := time.Now().UTC()
	admin, err := h.db.CreateUser(ctx, db.CreateUserParams{
		Username:  "admin",
		Email:     h.cfg.SeedAdminEmail,
		Role:      "admin",
		IsActive:  true,
		InvitedBy: pgtype.UUID{Valid: false},
		ConsentAt: consentAt,
	})
	if err != nil {
		return fmt.Errorf("seed admin: create user: %w", err)
	}

	if h.log != nil {
		h.log.Info("admin user created via SEED_ADMIN_EMAIL", "email", h.cfg.SeedAdminEmail)
	}

	// Send magic link so admin can log in immediately
	if err := h.sendMagicLinkToUser(ctx, admin, "login"); err != nil {
		return fmt.Errorf("seed admin: send magic link: %w", err)
	}

	if h.log != nil {
		h.log.Info("admin magic link sent", "email", h.cfg.SeedAdminEmail)
	}
	return nil
}
