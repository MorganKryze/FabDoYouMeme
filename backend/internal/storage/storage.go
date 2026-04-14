// backend/internal/storage/storage.go
package storage

import (
	"context"
	"io"
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

	// Upload streams body to the backend at key with the given content type and
	// exact size. Used when the browser cannot PUT directly due to CORS.
	Upload(ctx context.Context, key string, body io.Reader, contentType string, size int64) error

	// Download returns a stream for reading the object at key along with its
	// content-type and content-length (content-length may be 0 if unknown).
	// The caller MUST Close the returned reader. Used to proxy GETs through
	// the backend rather than redirecting the browser to an external URL.
	Download(ctx context.Context, key string) (body io.ReadCloser, contentType string, size int64, err error)

	// Delete removes the object at key. Non-fatal if key does not exist.
	Delete(ctx context.Context, key string) error

	// Probe checks connectivity to the storage backend.
	// Returns nil if the bucket is reachable.
	Probe(ctx context.Context) error
}

// ObjectKey returns the canonical storage key for an item version.
// Format: packs/{packID}/items/{itemID}/v{versionNumber}/{filename}
func ObjectKey(packID, itemID string, versionNumber int, filename string) string {
	return "packs/" + packID + "/items/" + itemID + "/v" + strconv.Itoa(versionNumber) + "/" + filename
}
