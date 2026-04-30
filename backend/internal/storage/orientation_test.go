// backend/internal/storage/orientation_test.go
package storage

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestDetectOrientation(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want string
	}{
		{"square_exact", 200, 200, OrientationSquare},
		{"landscape_5_4_snaps_4_3", 250, 200, OrientationLandscape4x3},
		{"landscape_4_3_exact", 800, 600, OrientationLandscape4x3},
		{"landscape_16_9_exact", 1920, 1080, OrientationLandscape16x9},
		{"landscape_3_2_snaps_4_3", 300, 200, OrientationLandscape4x3},
		{"portrait_3_4_exact", 600, 800, OrientationPortrait3x4},
		{"portrait_9_16_exact", 1080, 1920, OrientationPortrait9x16},
		{"portrait_2_3_snaps_3_4", 200, 300, OrientationPortrait3x4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectOrientation(pngOf(t, tc.w, tc.h))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("DetectOrientation(%dx%d) = %q, want %q", tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestDetectOrientation_InvalidBytes(t *testing.T) {
	if _, err := DetectOrientation([]byte("not an image")); err == nil {
		t.Fatal("expected error for non-image bytes")
	}
}
