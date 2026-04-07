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
