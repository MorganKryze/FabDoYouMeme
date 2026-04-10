package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// FakeStorage is an in-memory storage.Storage implementation for handler
// tests that don't want to spin up a MinIO container. It returns deterministic
// pre-signed URLs and tracks every Presign/Delete call so tests can assert on
// interactions.
//
// Tests that need real S3 behaviour (multipart, ACLs, content-disposition)
// should use MinIO via testcontainers instead — see the Stage 6 test plan.
type FakeStorage struct {
	mu            sync.Mutex
	Uploads       []string // keys that had a presigned upload URL generated
	Downloads     []string // keys that had a presigned download URL generated
	Deletes       []string // keys that were deleted
	UploadErr     error    // if non-nil, PresignUpload returns this
	DownloadErr   error    // if non-nil, PresignDownload returns this
	DeleteErr     error    // if non-nil, Delete returns this
	ProbeErr      error    // if non-nil, Probe returns this
	URLPrefix     string   // override for returned URLs (default "https://fake.storage")
}

// Ensure FakeStorage satisfies the storage.Storage interface at compile time.
var _ storage.Storage = (*FakeStorage)(nil)

// NewFakeStorage returns a FakeStorage with sensible defaults.
func NewFakeStorage() *FakeStorage {
	return &FakeStorage{URLPrefix: "https://fake.storage"}
}

func (f *FakeStorage) PresignUpload(_ context.Context, key string, _ time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.UploadErr != nil {
		return "", f.UploadErr
	}
	f.Uploads = append(f.Uploads, key)
	return f.URLPrefix + "/upload/" + key, nil
}

func (f *FakeStorage) PresignDownload(_ context.Context, key string, _ time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DownloadErr != nil {
		return "", f.DownloadErr
	}
	f.Downloads = append(f.Downloads, key)
	return f.URLPrefix + "/download/" + key, nil
}

func (f *FakeStorage) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	f.Deletes = append(f.Deletes, key)
	return nil
}

func (f *FakeStorage) Probe(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ProbeErr
}

// Reset clears recorded interactions and errors. Useful when re-using a fake
// across sub-tests.
func (f *FakeStorage) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Uploads = nil
	f.Downloads = nil
	f.Deletes = nil
	f.UploadErr = nil
	f.DownloadErr = nil
	f.DeleteErr = nil
	f.ProbeErr = nil
}
