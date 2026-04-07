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
