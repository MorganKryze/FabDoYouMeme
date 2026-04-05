// backend/internal/auth/email.go
package auth

import "context"

type LoginEmailData struct {
	Username      string
	MagicLinkURL  string
	FrontendURL   string
	ExpiryMinutes int
}

type EmailChangeData struct {
	Username      string
	MagicLinkURL  string
	FrontendURL   string
	ExpiryMinutes int
}

type EmailChangedNotificationData struct {
	Username       string
	NewEmailMasked string
	FrontendURL    string
}

// EmailSender is implemented by the email package (Phase 5).
type EmailSender interface {
	SendMagicLinkLogin(ctx context.Context, to string, data LoginEmailData) error
	SendMagicLinkEmailChange(ctx context.Context, to string, data EmailChangeData) error
	SendEmailChangedNotification(ctx context.Context, to string, data EmailChangedNotificationData) error
}
