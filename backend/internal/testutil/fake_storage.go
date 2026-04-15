package testutil

import (
	"context"
	"io"
	"strings"
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
// FakePurgeCall records one call to Purge: the prefix passed plus the
// count the fake returned. Tests assert on `len(Purges)` and on the
// prefix to prove the call happened with the expected scope.
type FakePurgeCall struct {
	Prefix string
	Count  int64
}

type FakeStorage struct {
	mu            sync.Mutex
	Uploads       []string        // keys that had a presigned upload URL generated
	DirectUploads []string        // keys that had a direct Upload() call
	Downloads     []string        // keys that had a presigned download URL generated
	Deletes       []string        // keys that were deleted
	Purges        []FakePurgeCall // recorded Purge calls
	UploadErr     error           // if non-nil, PresignUpload / Upload return this
	DownloadErr   error           // if non-nil, PresignDownload returns this
	DeleteErr     error           // if non-nil, Delete returns this
	PurgeErr      error           // if non-nil, Purge returns this
	PurgeCount    int64           // if >0, overrides the default count returned by Purge
	StatsErr      error           // if non-nil, Stats returns this
	StatsCount    int64           // overrides the object count returned by Stats
	StatsBytes    int64           // overrides the byte total returned by Stats
	ProbeErr      error           // if non-nil, Probe returns this
	URLPrefix     string          // override for returned URLs (default "https://fake.storage")
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

func (f *FakeStorage) Upload(_ context.Context, key string, body io.Reader, _ string, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.UploadErr != nil {
		return f.UploadErr
	}
	// Drain the body so the caller can't accidentally rely on short reads.
	if body != nil {
		_, _ = io.Copy(io.Discard, body)
	}
	f.DirectUploads = append(f.DirectUploads, key)
	return nil
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

func (f *FakeStorage) Download(_ context.Context, key string) (io.ReadCloser, string, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DownloadErr != nil {
		return nil, "", 0, f.DownloadErr
	}
	f.Downloads = append(f.Downloads, key)
	return io.NopCloser(strings.NewReader("")), "application/octet-stream", 0, nil
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

// Purge records the call and returns either PurgeCount (if set) or a
// synthetic count based on tracked uploads. Tests that want a specific
// count should set PurgeCount before calling the service.
func (f *FakeStorage) Purge(_ context.Context, prefix string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.PurgeErr != nil {
		return 0, f.PurgeErr
	}
	count := f.PurgeCount
	if count == 0 {
		count = int64(len(f.Uploads) + len(f.DirectUploads))
	}
	f.Purges = append(f.Purges, FakePurgeCall{Prefix: prefix, Count: count})
	return count, nil
}

// Stats returns the configured StatsCount / StatsBytes. Tests that care
// about the widget numbers should set these before invoking the handler;
// the default is zero/zero, matching an empty bucket.
func (f *FakeStorage) Stats(_ context.Context, _ string) (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.StatsErr != nil {
		return 0, 0, f.StatsErr
	}
	return f.StatsCount, f.StatsBytes, nil
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
	f.DirectUploads = nil
	f.Downloads = nil
	f.Deletes = nil
	f.Purges = nil
	f.UploadErr = nil
	f.DownloadErr = nil
	f.DeleteErr = nil
	f.PurgeErr = nil
	f.PurgeCount = 0
	f.StatsErr = nil
	f.StatsCount = 0
	f.StatsBytes = 0
	f.ProbeErr = nil
}
