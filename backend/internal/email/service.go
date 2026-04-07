// backend/internal/email/service.go
package email

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	ht "html/template"
	tt "text/template"

	gomail "github.com/wneessen/go-mail"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

//go:embed templates
var templateFS embed.FS

// Service implements auth.EmailSender using go-mail + embedded templates.
type Service struct {
	cfg      *config.Config
	htmlTmpl *ht.Template
	txtTmpl  *tt.Template
}

// NewService parses all embedded templates. Returns an error if any template has a syntax error.
func NewService(cfg *config.Config) (*Service, error) {
	htmlTmpl, err := ht.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("email: parse HTML templates: %w", err)
	}
	txtTmpl, err := tt.ParseFS(templateFS, "templates/*.txt")
	if err != nil {
		return nil, fmt.Errorf("email: parse text templates: %w", err)
	}
	return &Service{cfg: cfg, htmlTmpl: htmlTmpl, txtTmpl: txtTmpl}, nil
}

// Compile-time check that Service satisfies auth.EmailSender.
var _ auth.EmailSender = (*Service)(nil)

// RenderLogin renders the login email templates to strings (used in tests).
func (s *Service) RenderLogin(data auth.LoginEmailData) (html, text string, err error) {
	return s.render("magic_link_login.html", "magic_link_login.txt", data)
}

func (s *Service) render(htmlFile, txtFile string, data any) (html, text string, err error) {
	var hbuf, tbuf bytes.Buffer
	if err = s.htmlTmpl.Lookup(htmlFile).Execute(&hbuf, data); err != nil {
		return "", "", fmt.Errorf("render %s: %w", htmlFile, err)
	}
	if err = s.txtTmpl.Lookup(txtFile).Execute(&tbuf, data); err != nil {
		return "", "", fmt.Errorf("render %s: %w", txtFile, err)
	}
	return hbuf.String(), tbuf.String(), nil
}

func (s *Service) send(ctx context.Context, to, subject, htmlBody, txtBody string) error {
	tlsPolicy := gomail.TLSMandatory
	if s.cfg.SMTPPort == 1025 {
		// Dev mode (Mailpit): no TLS
		tlsPolicy = gomail.NoTLS
	}

	client, err := gomail.NewClient(s.cfg.SMTPHost,
		gomail.WithPort(s.cfg.SMTPPort),
		gomail.WithTLSPolicy(tlsPolicy),
		gomail.WithUsername(s.cfg.SMTPUsername),
		gomail.WithPassword(s.cfg.SMTPPassword),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
	)
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

// SendMagicLinkLogin implements auth.EmailSender.
func (s *Service) SendMagicLinkLogin(ctx context.Context, to string, data auth.LoginEmailData) error {
	html, txt, err := s.render("magic_link_login.html", "magic_link_login.txt", data)
	if err != nil {
		return err
	}
	return s.send(ctx, to, "Your FabDoYouMeme login link", html, txt)
}

// SendMagicLinkEmailChange implements auth.EmailSender.
func (s *Service) SendMagicLinkEmailChange(ctx context.Context, to string, data auth.EmailChangeData) error {
	html, txt, err := s.render("magic_link_email_change.html", "magic_link_email_change.txt", data)
	if err != nil {
		return err
	}
	return s.send(ctx, to, "Confirm your new email address", html, txt)
}

// SendEmailChangedNotification implements auth.EmailSender.
func (s *Service) SendEmailChangedNotification(ctx context.Context, to string, data auth.EmailChangedNotificationData) error {
	html, txt, err := s.render("notification_email_changed.html", "notification_email_changed.txt", data)
	if err != nil {
		return err
	}
	return s.send(ctx, to, "Your FabDoYouMeme email address was changed", html, txt)
}
