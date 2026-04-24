// backend/internal/email/service.go
package email

import (
	"bytes"
	"context"
	"embed"
	"fmt"

	gomail "github.com/wneessen/go-mail"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

//go:embed templates
var templateFS embed.FS

// supportedLocales is kept in sync with frontend/src/lib/i18n/locale.ts
// and the CHECK constraints in migration 011. Adding a locale = add
// templates/<code>/, add to this slice, add a CHECK migration, update
// frontend.
var supportedLocales = []string{"en", "fr"}

// Service implements auth.EmailSender using go-mail + embedded templates
// indexed by locale. The per-locale template tree is built once at
// construction; missing templates cause NewService to fail so mis-shipped
// locale bundles cannot silently fall back in production.
type Service struct {
	cfg  *config.Config
	tree map[string]*localeTemplates
}

// NewService parses all embedded templates for every supported locale.
// Returns an error if any locale is missing a template (parity check).
func NewService(cfg *config.Config) (*Service, error) {
	tree, err := loadTemplateTree(templateFS, supportedLocales)
	if err != nil {
		return nil, fmt.Errorf("email: %w", err)
	}
	return &Service{cfg: cfg, tree: tree}, nil
}

// Compile-time check that Service satisfies auth.EmailSender.
var _ auth.EmailSender = (*Service)(nil)

// RenderLogin renders the EN login email templates to strings (used in tests).
// Kept for backwards compatibility with existing test call sites.
func (s *Service) RenderLogin(data auth.LoginEmailData) (html, text string, err error) {
	return s.renderLocale("en", "magic_link_login", data)
}

// renderLocale executes the named template (no extension) in the requested
// locale. Unknown locale falls back to "en" — the parity check guarantees
// every template exists in en.
func (s *Service) renderLocale(locale, name string, data any) (html, text string, err error) {
	lt, ok := s.tree[locale]
	if !ok {
		lt = s.tree["en"]
	}
	var hbuf, tbuf bytes.Buffer
	if err = lt.HTML.Lookup(name+".html").Execute(&hbuf, data); err != nil {
		return "", "", fmt.Errorf("render %s.html (%s): %w", name, locale, err)
	}
	if err = lt.Text.Lookup(name+".txt").Execute(&tbuf, data); err != nil {
		return "", "", fmt.Errorf("render %s.txt (%s): %w", name, locale, err)
	}
	return hbuf.String(), tbuf.String(), nil
}

// subject returns the localized subject line, falling back to EN if the
// requested locale lacks the key.
func (s *Service) subject(locale, name string) string {
	if lt, ok := s.tree[locale]; ok {
		if subj, ok := lt.Subjects[name]; ok {
			return subj
		}
	}
	if en, ok := s.tree["en"]; ok {
		if subj, ok := en.Subjects[name]; ok {
			return subj
		}
	}
	return name
}

// buildClientOptions returns the go-mail options derived from the current
// SMTP config. The TLS strategy is dispatched by port so both plaintext dev
// (Mailpit) and implicit-TLS providers (OVH SMTPS on 465) work alongside the
// STARTTLS default on 587.
func (s *Service) buildClientOptions() []gomail.Option {
	opts := []gomail.Option{gomail.WithPort(s.cfg.SMTPPort)}

	switch s.cfg.SMTPPort {
	case 1025:
		// Mailpit / local dev — plaintext, no TLS.
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	case 465:
		// SMTPS / implicit TLS — the server expects a TLS ClientHello
		// immediately after TCP connect, before any SMTP bytes. STARTTLS
		// on this port would hang until the server closes the connection.
		opts = append(opts, gomail.WithSSL())
	default:
		// Submission port (587) and anything else — upgrade via STARTTLS.
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	}

	if s.cfg.SMTPUsername != "" {
		opts = append(opts,
			gomail.WithUsername(s.cfg.SMTPUsername),
			gomail.WithPassword(s.cfg.SMTPPassword),
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
		)
	}
	return opts
}

func (s *Service) send(ctx context.Context, to, subject, htmlBody, txtBody string) error {
	client, err := gomail.NewClient(s.cfg.SMTPHost, s.buildClientOptions()...)
	if err != nil {
		return fmt.Errorf("email: new client: %w", err)
	}

	m := gomail.NewMsg()
	if err := m.From(s.cfg.SMTPFrom); err != nil {
		return fmt.Errorf("email: set from: %w", err)
	}
	if err := m.To(to); err != nil {
		return fmt.Errorf("email: set to: %w", err)
	}
	m.Subject(subject)
	m.SetBodyString(gomail.TypeTextHTML, htmlBody)
	m.AddAlternativeString(gomail.TypeTextPlain, txtBody)
	return client.DialAndSend(m)
}

// Probe verifies SMTP reachability without sending any mail.
// It opens a client with the same TLS/auth options used by send(), dials with
// the given context, and closes. Returns nil on success, a wrapped error otherwise.
func (s *Service) Probe(ctx context.Context) error {
	client, err := gomail.NewClient(s.cfg.SMTPHost, s.buildClientOptions()...)
	if err != nil {
		return fmt.Errorf("smtp probe: new client: %w", err)
	}
	if err := client.DialWithContext(ctx); err != nil {
		return fmt.Errorf("smtp probe: dial: %w", err)
	}
	_ = client.Close()
	return nil
}

// SendMagicLinkLogin implements auth.EmailSender.
func (s *Service) SendMagicLinkLogin(ctx context.Context, to, locale string, data auth.LoginEmailData) error {
	html, txt, err := s.renderLocale(locale, "magic_link_login", data)
	if err != nil {
		return err
	}
	return s.send(ctx, to, s.subject(locale, "magic_link_login"), html, txt)
}

// SendMagicLinkEmailChange implements auth.EmailSender.
func (s *Service) SendMagicLinkEmailChange(ctx context.Context, to, locale string, data auth.EmailChangeData) error {
	html, txt, err := s.renderLocale(locale, "magic_link_email_change", data)
	if err != nil {
		return err
	}
	return s.send(ctx, to, s.subject(locale, "magic_link_email_change"), html, txt)
}

// SendEmailChangedNotification implements auth.EmailSender.
func (s *Service) SendEmailChangedNotification(ctx context.Context, to, locale string, data auth.EmailChangedNotificationData) error {
	html, txt, err := s.renderLocale(locale, "notification_email_changed", data)
	if err != nil {
		return err
	}
	return s.send(ctx, to, s.subject(locale, "notification_email_changed"), html, txt)
}
