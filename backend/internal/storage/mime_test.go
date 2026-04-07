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
