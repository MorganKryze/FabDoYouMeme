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

// EmailSender is implemented by the email package.
// Every method takes an explicit `locale` so callers thread through the
// recipient's language (users.locale, invites.locale, or cfg.DefaultLocale).
type EmailSender interface {
	SendMagicLinkLogin(ctx context.Context, to, locale string, data LoginEmailData) error
	SendMagicLinkEmailChange(ctx context.Context, to, locale string, data EmailChangeData) error
	SendEmailChangedNotification(ctx context.Context, to, locale string, data EmailChangedNotificationData) error
}
