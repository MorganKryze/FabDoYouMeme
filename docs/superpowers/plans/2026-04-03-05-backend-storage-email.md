# Backend — Storage (RustFS) + Email — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `Storage` interface (S3-compatible RustFS client with pre-signed URL generation and MIME validation) and the `EmailService` (go-mail SMTP sender with html/template rendering). These two packages are consumed by Phase 4 auth handlers and Phase 7 REST API handlers.

**Architecture:** `internal/storage/` wraps `aws-sdk-go-v2/s3` behind a `Storage` interface — swap implementations without touching call sites. `internal/email/` implements `auth.EmailSender` using go-mail and embedded HTML/text templates. No circular imports: email imports auth for data types, auth does not import email.

**Tech Stack:** `aws-sdk-go-v2`, `github.com/wneessen/go-mail@v0.4.2`, `html/template`, `text/template`, `embed`.

**Prerequisite:** Phase 3 complete (config struct available). Phase 4 complete (`auth.EmailSender` interface defined).

---

### Task 1: Storage interface + MIME validator

**Files:**

- Create: `backend/internal/storage/storage.go`
- Create: `backend/internal/storage/mime.go`
- Create: `backend/internal/storage/mime_test.go`

- [ ] **Step 1: Write MIME validation tests**

```go
// backend/internal/storage/mime_test.go
package storage_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("pngBytes: %v", err)
	}
	return buf.Bytes()
}

func TestValidateMIME_PNG_OK(t *testing.T) {
	if err := storage.ValidateMIME("image/png", pngBytes(t)); err != nil {
		t.Errorf("expected no error for valid PNG: %v", err)
	}
}

func TestValidateMIME_WrongDeclared(t *testing.T) {
	// File is PNG but declared as JPEG
	if err := storage.ValidateMIME("image/jpeg", pngBytes(t)); err == nil {
		t.Error("expected error when declared MIME does not match magic bytes")
	}
}

func TestValidateMIME_NotAllowed(t *testing.T) {
	if err := storage.ValidateMIME("application/pdf", pngBytes(t)); err == nil {
		t.Error("expected error for disallowed MIME type")
	}
}

func TestValidateMIME_InvalidBytes(t *testing.T) {
	garbage := []byte{0x00, 0x01, 0x02, 0x03}
	if err := storage.ValidateMIME("image/png", garbage); err == nil {
		t.Error("expected error for non-image bytes")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/storage/... -run TestValidateMIME -v
```

Expected: compile error (package does not exist yet).

- [ ] **Step 3: Write `storage.go` — interface**

```go
// backend/internal/storage/storage.go
package storage

import (
	"context"
	"strconv"
	"time"
)

// Storage is the interface that wraps RustFS/S3 operations.
// The concrete implementation is S3Storage; tests may use a stub.
type Storage interface {
	// PresignUpload returns a pre-signed PUT URL valid for the given TTL.
	// The caller must validate MIME type and size before calling.
	PresignUpload(ctx context.Context, key string, ttl time.Duration) (string, error)

	// PresignDownload returns a pre-signed GET URL with response-content-disposition=attachment.
	PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error)

	// Delete removes the object at key. Non-fatal if key does not exist.
	Delete(ctx context.Context, key string) error
}

// ObjectKey returns the canonical storage key for an item version.
// Format: packs/{packID}/items/{itemID}/v{versionNumber}/{filename}
func ObjectKey(packID, itemID string, versionNumber int, filename string) string {
	return "packs/" + packID + "/items/" + itemID + "/v" + strconv.Itoa(versionNumber) + "/" + filename
}
```

- [ ] **Step 4: Write `mime.go`**

```go
// backend/internal/storage/mime.go
package storage

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	// WebP decoding support
	_ "golang.org/x/image/webp"
)

// allowedMIMEs is the MIME-type allowlist for uploaded assets.
var allowedMIMEs = map[string]string{
	"image/jpeg": "jpeg",
	"image/png":  "png",
	"image/webp": "webp",
}

// ValidateMIME checks that:
//  1. declaredMIME is in the allowlist (JPEG, PNG, WebP).
//  2. The magic bytes in sample match the declared type.
//
// sample should be the first ~512 bytes of the file (more is fine).
func ValidateMIME(declaredMIME string, sample []byte) error {
	expectedFormat, ok := allowedMIMEs[declaredMIME]
	if !ok {
		return fmt.Errorf("MIME type %q is not allowed (accepted: image/jpeg, image/png, image/webp)", declaredMIME)
	}

	_, detectedFormat, err := image.DecodeConfig(bytes.NewReader(sample))
	if err != nil {
		return fmt.Errorf("invalid image data: %w", err)
	}

	if detectedFormat != expectedFormat {
		return fmt.Errorf("magic byte mismatch: declared %q but file appears to be %q", declaredMIME, "image/"+detectedFormat)
	}

	return nil
}
```

- [ ] **Step 5: Add `golang.org/x/image` dependency**

```bash
cd backend && go get golang.org/x/image@latest
```

- [ ] **Step 6: Run tests**

```bash
cd backend && go test ./internal/storage/... -run TestValidateMIME -v
```

Expected: all `PASS`.

---

### Task 2: S3/RustFS client implementation

**Files:**

- Create: `backend/internal/storage/s3.go`

- [ ] **Step 1: Write `s3.go`**

```go
// backend/internal/storage/s3.go
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage is the concrete Storage implementation backed by an S3-compatible store (RustFS).
type S3Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

// NewS3 builds an S3Storage pointed at the given endpoint.
// UsePathStyle is forced on — required by RustFS.
func NewS3(endpoint, accessKey, secretKey, bucket string) (*S3Storage, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, fmt.Errorf("storage: endpoint, accessKey, secretKey, and bucket are required")
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"), // RustFS ignores region value
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // RustFS requires path-style, not virtual-hosted-style
	})

	return &S3Storage{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    bucket,
	}, nil
}

// PresignUpload returns a pre-signed PUT URL valid for ttl.
func (s *S3Storage) PresignUpload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign upload %q: %w", key, err)
	}
	return req.URL, nil
}

// PresignDownload returns a pre-signed GET URL with attachment disposition.
func (s *S3Storage) PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String("attachment"),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign download %q: %w", key, err)
	}
	return req.URL, nil
}

// Delete removes the object at key. Returns nil if the key does not exist.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

// Compile-time interface check
var _ Storage = (*S3Storage)(nil)
```

- [ ] **Step 2: Build check**

```bash
cd backend && go build ./internal/storage/...
```

Expected: no errors.

---

### Task 3: Email templates

**Files:**

- Create: `backend/internal/email/templates/magic_link_login.html`
- Create: `backend/internal/email/templates/magic_link_login.txt`
- Create: `backend/internal/email/templates/magic_link_email_change.html`
- Create: `backend/internal/email/templates/magic_link_email_change.txt`
- Create: `backend/internal/email/templates/notification_email_changed.html`
- Create: `backend/internal/email/templates/notification_email_changed.txt`

- [ ] **Step 1: Write `magic_link_login.html`**

```html
<!-- backend/internal/email/templates/magic_link_login.html -->
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>Log in to FabDoYouMeme</title>
  </head>
  <body
    style="font-family:sans-serif;max-width:560px;margin:0 auto;padding:24px"
  >
    <h2>Hi {{.Username}},</h2>
    <p>
      Click the button below to log in to FabDoYouMeme. This link expires in
      <strong>{{.ExpiryMinutes}} minutes</strong> and can only be used once.
    </p>
    <p style="text-align:center;margin:32px 0">
      <a
        href="{{.MagicLinkURL}}"
        style="background:#18181b;color:#fff;padding:12px 28px;border-radius:6px;text-decoration:none;font-weight:600"
      >
        Log In →
      </a>
    </p>
    <p style="color:#71717a;font-size:14px">
      If you didn't request this, you can safely ignore this email.
    </p>
    <hr style="border:none;border-top:1px solid #e4e4e7;margin:24px 0" />
    <p style="color:#71717a;font-size:12px">
      <a href="{{.FrontendURL}}" style="color:#71717a">FabDoYouMeme</a>
    </p>
  </body>
</html>
```

- [ ] **Step 2: Write `magic_link_login.txt`**

```text
Hi {{.Username}},

Use this link to log in to FabDoYouMeme (expires in {{.ExpiryMinutes}} minutes, one-time use):

{{.MagicLinkURL}}

If you didn't request this, ignore this email.

-- FabDoYouMeme: {{.FrontendURL}}
```

- [ ] **Step 3: Write `magic_link_email_change.html`**

```html
<!-- backend/internal/email/templates/magic_link_email_change.html -->
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>Confirm your new email — FabDoYouMeme</title>
  </head>
  <body
    style="font-family:sans-serif;max-width:560px;margin:0 auto;padding:24px"
  >
    <h2>Hi {{.Username}},</h2>
    <p>
      Someone requested to change your FabDoYouMeme account email to this
      address. Click below to confirm. This link expires in
      <strong>{{.ExpiryMinutes}} minutes</strong>.
    </p>
    <p style="text-align:center;margin:32px 0">
      <a
        href="{{.MagicLinkURL}}"
        style="background:#18181b;color:#fff;padding:12px 28px;border-radius:6px;text-decoration:none;font-weight:600"
      >
        Confirm Email Change →
      </a>
    </p>
    <p style="color:#71717a;font-size:14px">
      If you didn't request this, your account is safe — the change will not
      take effect. Your current email address is still active.
    </p>
    <hr style="border:none;border-top:1px solid #e4e4e7;margin:24px 0" />
    <p style="color:#71717a;font-size:12px">
      <a href="{{.FrontendURL}}" style="color:#71717a">FabDoYouMeme</a>
    </p>
  </body>
</html>
```

- [ ] **Step 4: Write `magic_link_email_change.txt`**

```text
Hi {{.Username}},

Someone requested to change your FabDoYouMeme account email to this address.
Click this link to confirm (expires in {{.ExpiryMinutes}} minutes):

{{.MagicLinkURL}}

If you didn't request this, your account is safe — the change will not take effect.
Your current email address is still active.

-- FabDoYouMeme: {{.FrontendURL}}
```

- [ ] **Step 5: Write `notification_email_changed.html`**

```html
<!-- backend/internal/email/templates/notification_email_changed.html -->
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>Email address changed — FabDoYouMeme</title>
  </head>
  <body
    style="font-family:sans-serif;max-width:560px;margin:0 auto;padding:24px"
  >
    <h2>Hi {{.Username}},</h2>
    <p>
      Your FabDoYouMeme account email was changed to
      <strong>{{.NewEmailMasked}}</strong>.
    </p>
    <p>If you made this change, no action is needed.</p>
    <p style="color:#dc2626">
      If you did <strong>NOT</strong> make this change, contact your admin
      immediately.
    </p>
    <hr style="border:none;border-top:1px solid #e4e4e7;margin:24px 0" />
    <p style="color:#71717a;font-size:12px">
      <a href="{{.FrontendURL}}" style="color:#71717a">FabDoYouMeme</a>
    </p>
  </body>
</html>
```

- [ ] **Step 6: Write `notification_email_changed.txt`**

```text
Hi {{.Username}},

Your FabDoYouMeme account email was changed to {{.NewEmailMasked}}.

If you made this change, no action is needed.

If you did NOT make this change, contact your admin immediately.

-- FabDoYouMeme: {{.FrontendURL}}
```

---

### Task 4: Email service

**Files:**

- Create: `backend/internal/email/service.go`
- Create: `backend/internal/email/service_test.go`

- [ ] **Step 1: Write the template rendering test (no SMTP required)**

```go
// backend/internal/email/service_test.go
package email_test

import (
	"context"
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

// TestRenderOnly verifies template rendering without sending.
// Use email.RenderLogin for testing the output.
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
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/email/... -run TestNewService -run TestRenderLogin -v
```

Expected: compile error (package does not exist yet).

- [ ] **Step 3: Implement `service.go`**

```go
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
	m.SetBodyHTMLTemplate(s.htmlTmpl.Lookup("_body"), nil) // placeholder; overridden below
	// Set body directly from rendered strings
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
```

Note: the `send` method above used a placeholder for `SetBodyHTMLTemplate` then overrides it. Remove the placeholder lines and use `SetBodyString` directly — simplify `send` to:

```go
func (s *Service) send(ctx context.Context, to, subject, htmlBody, txtBody string) error {
	tlsPolicy := gomail.TLSMandatory
	if s.cfg.SMTPPort == 1025 {
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
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/email/... -run TestNewService -run TestRenderLogin -v
```

Expected: all `PASS`.

- [ ] **Step 5: Full build check**

```bash
cd backend && go build ./...
```

Expected: no errors.

---

### Verification

```bash
cd backend && go test ./internal/storage/... ./internal/email/... -v
cd backend && go build ./...
```

Expected: all tests pass, build succeeds.

Mark phase 5 complete in `docs/implementation-status.md`.
