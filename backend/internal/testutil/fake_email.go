package testutil

import (
	"context"
	"sync"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
)

// FakeEmailPurpose identifies which SendXxx method a fake captured.
type FakeEmailPurpose string

const (
	FakeEmailLogin         FakeEmailPurpose = "login"
	FakeEmailChange        FakeEmailPurpose = "email_change"
	FakeEmailChangedNotify FakeEmailPurpose = "email_changed_notification"
)

// FakeEmailMsg records a single SendXxx call for later assertions. Only one of
// LoginData / ChangeData / NotifyData is populated depending on Purpose.
type FakeEmailMsg struct {
	To         string
	Purpose    FakeEmailPurpose
	LoginData  auth.LoginEmailData
	ChangeData auth.EmailChangeData
	NotifyData auth.EmailChangedNotificationData
}

// FakeEmail is an in-memory auth.EmailSender. Every Send captures its inputs
// into Sent so tests can assert on MagicLinkURL, Username, etc.
type FakeEmail struct {
	mu       sync.Mutex
	Sent     []FakeEmailMsg
	SendFail error // if non-nil, every Send returns this error and records nothing
}

// Ensure FakeEmail satisfies the auth.EmailSender interface at compile time.
var _ auth.EmailSender = (*FakeEmail)(nil)

// NewFakeEmail returns an empty FakeEmail.
func NewFakeEmail() *FakeEmail { return &FakeEmail{} }

func (f *FakeEmail) SendMagicLinkLogin(_ context.Context, to string, data auth.LoginEmailData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SendFail != nil {
		return f.SendFail
	}
	f.Sent = append(f.Sent, FakeEmailMsg{To: to, Purpose: FakeEmailLogin, LoginData: data})
	return nil
}

func (f *FakeEmail) SendMagicLinkEmailChange(_ context.Context, to string, data auth.EmailChangeData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SendFail != nil {
		return f.SendFail
	}
	f.Sent = append(f.Sent, FakeEmailMsg{To: to, Purpose: FakeEmailChange, ChangeData: data})
	return nil
}

func (f *FakeEmail) SendEmailChangedNotification(_ context.Context, to string, data auth.EmailChangedNotificationData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SendFail != nil {
		return f.SendFail
	}
	f.Sent = append(f.Sent, FakeEmailMsg{To: to, Purpose: FakeEmailChangedNotify, NotifyData: data})
	return nil
}

// LastLogin returns the most recent login email's data, or a zero value and
// false if no login email has been captured.
func (f *FakeEmail) LastLogin() (auth.LoginEmailData, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.Sent) - 1; i >= 0; i-- {
		if f.Sent[i].Purpose == FakeEmailLogin {
			return f.Sent[i].LoginData, true
		}
	}
	return auth.LoginEmailData{}, false
}

// Reset clears captured messages and any SendFail override.
func (f *FakeEmail) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Sent = nil
	f.SendFail = nil
}
