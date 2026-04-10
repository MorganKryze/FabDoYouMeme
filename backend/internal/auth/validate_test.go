// backend/internal/auth/validate_test.go
package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
)

// TestAuth_ValidateUsernameTable is the P2.9 acceptance test (finding 5.E).
// The review called out three concrete attack classes the DB-only validation
// failed to prevent: Unicode homoglyph (`аdmin` vs `admin`), RTL override
// renaming, and oversized rows. Every row below maps to at least one of
// those classes or to the straightforward length/shape rules.
func TestAuth_ValidateUsernameTable(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		// If wantErr, contains is optionally checked against the error message
		// so a future rule change that still rejects the input but under a
		// different category is caught.
		contains string
	}{
		{name: "ok simple lowercase", input: "alice"},
		{name: "ok mixed case digits", input: "Bob42"},
		{name: "ok underscore and hyphen", input: "a_b-c"},
		{name: "ok min length", input: "abc"},
		{name: "ok max length", input: strings.Repeat("a", 30)},

		{name: "reject empty", input: "", wantErr: true, contains: "3–30"},
		{name: "reject too short", input: "ab", wantErr: true, contains: "3–30"},
		{name: "reject too long", input: strings.Repeat("a", 31), wantErr: true, contains: "3–30"},
		{name: "reject space", input: "al ice", wantErr: true, contains: "ASCII"},
		{name: "reject dot", input: "al.ice", wantErr: true, contains: "ASCII"},
		{name: "reject at sign", input: "al@ice", wantErr: true, contains: "ASCII"},
		{name: "reject slash", input: "al/ice", wantErr: true, contains: "ASCII"},
		{name: "reject tab", input: "al\tice", wantErr: true, contains: "ASCII"},

		// Homoglyph + RTL: the attacks 5.E specifically called out.
		{name: "reject cyrillic homoglyph admin", input: "\u0430dmin", wantErr: true, contains: "ASCII"},
		{name: "reject RTL override", input: "user\u202Egro", wantErr: true, contains: "ASCII"},
		{name: "reject zero-width space", input: "al\u200Bice", wantErr: true, contains: "ASCII"},
		{name: "reject emoji", input: "fire\U0001F525", wantErr: true, contains: "ASCII"},

		// Control characters.
		{name: "reject NUL", input: "al\x00ice", wantErr: true, contains: "ASCII"},
		{name: "reject newline", input: "al\nice", wantErr: true, contains: "ASCII"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := auth.ValidateUsername(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error for %q, got nil", tc.input)
				}
				if !errors.Is(err, auth.ErrInvalidUsername) {
					t.Fatalf("want ErrInvalidUsername, got %v", err)
				}
				if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
					t.Fatalf("want error containing %q, got %q", tc.contains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("want no error for %q, got %v", tc.input, err)
			}
		})
	}
}

// TestAuth_ValidateEmailTable exercises the same three concerns for emails:
// required, size-bounded, and shape-checked. The display-form rejection
// prevents `"Admin" <admin@foo>` from being accepted by mail.ParseAddress
// and then rendered back to other users as just "Admin".
func TestAuth_ValidateEmailTable(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ok simple", input: "user@example.com"},
		{name: "ok with plus tag", input: "user+tag@example.com"},
		{name: "ok with subdomain", input: "user@mail.example.co.uk"},

		{name: "reject empty", input: "", wantErr: true},
		{name: "reject no at sign", input: "userexample.com", wantErr: true},
		{name: "reject whitespace only", input: "   ", wantErr: true},
		// 250 + 6 = 256 bytes → above the 254-byte cap.
		{name: "reject over 254 bytes", input: strings.Repeat("a", 250) + "@b.com", wantErr: true},
		{name: "reject display form", input: `"Display" <foo@bar.com>`, wantErr: true},
		{name: "reject angle-bracket wrap", input: "<foo@bar.com>", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := auth.ValidateEmail(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error for %q, got nil", tc.input)
				}
				if !errors.Is(err, auth.ErrInvalidEmail) {
					t.Fatalf("want ErrInvalidEmail, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("want no error for %q, got %v", tc.input, err)
			}
		})
	}
}
