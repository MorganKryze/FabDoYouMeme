// backend/internal/auth/bootstrap.go
package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// SeedAdmin creates the first admin user if SEED_ADMIN_EMAIL is set and no user
// with that email exists yet. Idempotent: safe to call on every startup.
func (h *Handler) SeedAdmin(ctx context.Context) error {
	if h.cfg.SeedAdminEmail == "" {
		return nil
	}

	// Check if the user already exists (idempotency).
	existing, err := h.db.GetUserByEmail(ctx, h.cfg.SeedAdminEmail)
	if err == nil {
		// User exists — do not modify the role, but ensure the
		// is_protected flag is stamped. This handles the first boot after
		// migration 006: the existing admin row pre-dates the column and
		// would otherwise keep the default false.
		if existing.Role != "admin" && h.log != nil {
			h.log.Warn("SEED_ADMIN_EMAIL is set but the account is not admin role; skipping bootstrap",
				"email", h.cfg.SeedAdminEmail)
			return nil
		}
		if !existing.IsProtected {
			if err := h.db.SetUserProtected(ctx, db.SetUserProtectedParams{
				ID: existing.ID, IsProtected: true,
			}); err != nil {
				return fmt.Errorf("seed admin: stamp protection: %w", err)
			}
			if h.log != nil {
				h.log.Info("bootstrap admin protection stamped", "email", h.cfg.SeedAdminEmail)
			}
		}
		return nil
	}

	// Create admin user with legitimate-interest consent_at (Art. 6(1)(f))
	consentAt := h.clock.Now().UTC()
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

	// Stamp protection on the fresh row. Must happen before any magic-link
	// email — if that fails, the protection is still in place.
	if err := h.db.SetUserProtected(ctx, db.SetUserProtectedParams{
		ID: admin.ID, IsProtected: true,
	}); err != nil {
		return fmt.Errorf("seed admin: stamp protection: %w", err)
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
