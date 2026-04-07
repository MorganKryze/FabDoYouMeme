// backend/internal/email/service_test.go
package email_test

import (
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/email"
)

func newTestService(t *testing.T) *email.Service {
	t.Helper()
	cfg := &config.Config{
		SMTPHost:     "localhost",
		SMTPPort:     1025, // Mailpit dev port
		SMTPUsername: "",
		SMTPPassword: "",
		SMTPFrom:     "noreply@test.local",
		FrontendURL:  "http://localhost:3000",
	}
	svc, err := email.NewService(cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestNewService_ParsesTemplates(t *testing.T) {
	// Just constructing the service exercises template parsing.
	// Any template syntax error causes NewService to return an error.
	newTestService(t)
}

// TestRenderLogin_ContainsMagicURL verifies template rendering without sending.
func TestRenderLogin_ContainsMagicURL(t *testing.T) {
	svc := newTestService(t)
	data := auth.LoginEmailData{
		Username:      "alice",
		MagicLinkURL:  "http://localhost:3000/auth/verify?token=abc123",
		FrontendURL:   "http://localhost:3000",
		ExpiryMinutes: 15,
	}
	html, txt, err := svc.RenderLogin(data)
	if err != nil {
		t.Fatalf("RenderLogin: %v", err)
	}
	if !contains(html, "abc123") {
		t.Error("HTML template missing magic URL")
	}
	if !contains(txt, "abc123") {
		t.Error("text template missing magic URL")
	}
	if !contains(html, "alice") {
		t.Error("HTML template missing username")
	}
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
