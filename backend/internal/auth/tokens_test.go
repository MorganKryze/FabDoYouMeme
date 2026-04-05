// backend/internal/auth/tokens_test.go
package auth_test

import (
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
)

func TestHashToken_Deterministic(t *testing.T) {
	h1 := auth.HashToken("my-token")
	h2 := auth.HashToken("my-token")
	if h1 != h2 {
		t.Errorf("hash is not deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 char hash, got %d", len(h1))
	}
}

func TestHashToken_Different(t *testing.T) {
	if auth.HashToken("token-a") == auth.HashToken("token-b") {
		t.Error("different tokens should produce different hashes")
	}
}

func TestGenerateRawToken_UniqueAndLen(t *testing.T) {
	t1, err := auth.GenerateRawToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t2, err := auth.GenerateRawToken()
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if t1 == t2 {
		t.Error("two generated tokens should not be equal")
	}
	// 32 bytes hex-encoded = 64 chars
	if len(t1) != 64 {
		t.Errorf("expected 64 char token, got %d", len(t1))
	}
}
