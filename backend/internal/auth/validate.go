// backend/internal/auth/validate.go
//
// Input validation for user-supplied identifiers. The DB enforces only
// uniqueness, not shape — before P2.9 (finding 5.E in the 2026-04-10
// review) an attacker could register `аdmin` (Cyrillic а) as a
// homoglyph of `admin`, embed a `\u202E` RTL override in a username,
// or hand the system a 10KB email that would break SMTP downstream.
// All rejected here, early, with specific error codes.
package auth

import (
	"errors"
	"net/mail"
)

// ErrInvalidUsername and ErrInvalidEmail are sentinel errors returned by
// ValidateUsername / ValidateEmail. Callers can use errors.Is to branch on
// the category without parsing the human message (the message stays as
// `err.Error()` for display-to-user paths and test assertions).
var (
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidEmail    = errors.New("invalid email")
)

// ValidateUsername enforces the wire-level contract for user-supplied
// usernames:
//
//  1. 3–30 bytes (because the charset is ASCII, bytes == characters)
//  2. ASCII letters, digits, underscore, or hyphen — nothing else
//
// The ASCII restriction is intentional. The review's original suggestion
// used unicode.IsLetter, but that re-admits the exact Cyrillic homoglyph
// attack the rule is meant to prevent. ASCII-only eliminates both
// homoglyphs and RTL overrides in one check.
func ValidateUsername(u string) error {
	if len(u) < 3 || len(u) > 30 {
		return wrapUsername("must be 3–30 characters")
	}
	for i := 0; i < len(u); i++ {
		c := u[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-':
		default:
			return wrapUsername("may contain only ASCII letters, digits, _ and -")
		}
	}
	return nil
}

// ValidateEmail enforces:
//
//  1. Non-empty
//  2. ≤254 bytes (RFC 5321 §4.5.3.1.3 path limit — the practical ceiling
//     for well-formed addresses; anything longer is almost certainly
//     hostile or broken)
//  3. Parseable via `net/mail.ParseAddress`
//  4. Bare address form only — no `"Display Name" <foo@bar>` tricks
func ValidateEmail(e string) error {
	if e == "" {
		return wrapEmail("is required")
	}
	if len(e) > 254 {
		return wrapEmail("exceeds 254 characters")
	}
	addr, err := mail.ParseAddress(e)
	if err != nil {
		return wrapEmail("is not a valid address")
	}
	if addr.Address != e {
		return wrapEmail("must be a bare address, not a display form")
	}
	return nil
}

func wrapUsername(msg string) error {
	return &validationError{sentinel: ErrInvalidUsername, msg: "username " + msg}
}

func wrapEmail(msg string) error {
	return &validationError{sentinel: ErrInvalidEmail, msg: "email " + msg}
}

// validationError keeps the human-facing message and the sentinel in one
// value. errors.Is(err, ErrInvalidUsername) works because Unwrap() returns
// the sentinel, and err.Error() is safe to render to clients because it
// contains no user input — only the field name and the rule that failed.
type validationError struct {
	sentinel error
	msg      string
}

func (v *validationError) Error() string { return v.msg }
func (v *validationError) Unwrap() error { return v.sentinel }
