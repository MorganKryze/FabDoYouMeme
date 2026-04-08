// backend/internal/auth/bootstrap_test.go

package auth_test

import (
	"context"
	"testing"
)

func TestSeedAdmin_CreatesAdminUser(t *testing.T) {
	// SeedAdmin creates the admin on first call and is a no-op on subsequent calls.
	// Both behaviours are tested here because SeedAdmin hardcodes username="admin",
	// so two separate tests would conflict on the unique constraint.
	h, q := newTestHandler(t)
	h.SetSeedAdminEmail("bootstrap_admin@test.com")

	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}

	user, err := q.GetUserByEmail(context.Background(), "bootstrap_admin@test.com")
	if err != nil {
		t.Fatalf("user not found: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected role=admin, got %s", user.Role)
	}

	// Idempotency: second call must find the existing user and return nil.
	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("second SeedAdmin (idempotency check): %v", err)
	}
}

func TestSeedAdmin_NoEmail_IsNoop(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetSeedAdminEmail("") // empty — must be no-op

	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("SeedAdmin with no email: %v", err)
	}
}
