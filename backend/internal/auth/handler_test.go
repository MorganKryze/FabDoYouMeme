// backend/internal/auth/handler_test.go
package auth

import "testing"

func TestMaskEmail_Normal(t *testing.T) {
	got := maskEmail("user@example.com")
	if got != "***@example.com" {
		t.Errorf("got %q, want %q", got, "***@example.com")
	}
}

func TestMaskEmail_NoAt(t *testing.T) {
	got := maskEmail("notanemail")
	if got != "***" {
		t.Errorf("got %q, want %q", got, "***")
	}
}

func TestMaskEmail_EmptyLocalPart(t *testing.T) {
	got := maskEmail("@example.com")
	if got != "***@example.com" {
		t.Errorf("got %q, want %q", got, "***@example.com")
	}
}
