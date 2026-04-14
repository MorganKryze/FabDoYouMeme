// backend/internal/email/service_test.go
package email_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

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

// TestProbe_DialFailure: a closed port produces a wrapped "smtp probe" error.
// Happy-path probing is covered implicitly by the Mailpit dev environment;
// unit-testing it here would require a TLS-capable fake server since Probe
// only takes the NoTLS branch when SMTPPort == 1025.
func TestProbe_DialFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close() // free the port so the dial fails at TCP level

	svc, err := email.NewService(&config.Config{
		SMTPHost:    "127.0.0.1",
		SMTPPort:    port,
		SMTPFrom:    "noreply@test.local",
		FrontendURL: "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err = svc.Probe(ctx)
	if err == nil {
		t.Fatalf("Probe against closed port: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "smtp probe") {
		t.Errorf("expected error to contain 'smtp probe', got %q", err.Error())
	}
}

// TestProbe_ContextCancellation: a listener that accepts but never speaks
// causes Probe to hit the context deadline while waiting for the greeting.
func TestProbe_ContextCancellation(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Hold the connection open without ever writing a greeting.
		defer conn.Close()
		time.Sleep(2 * time.Second)
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	svc, err := email.NewService(&config.Config{
		SMTPHost:    "127.0.0.1",
		SMTPPort:    port,
		SMTPFrom:    "noreply@test.local",
		FrontendURL: "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := svc.Probe(ctx); err == nil {
		t.Fatalf("Probe against non-responding server: expected error, got nil")
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
