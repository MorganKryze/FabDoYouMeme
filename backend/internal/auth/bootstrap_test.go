// backend/internal/auth/bootstrap_test.go
//go:build integration

package auth_test

import (
	"context"
	"testing"
)

func TestSeedAdmin_CreatesAdminUser(t *testing.T) {
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
}

func TestSeedAdmin_Idempotent(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetSeedAdminEmail("idempotent_admin@test.com")

	// Run twice — must not error or create duplicates
	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("first SeedAdmin: %v", err)
	}
	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("second SeedAdmin: %v", err)
	}
}

func TestSeedAdmin_NoEmail_IsNoop(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetSeedAdminEmail("") // empty — must be no-op

	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("SeedAdmin with no email: %v", err)
	}
}
