// backend/internal/storage/orientation.go
package storage

import (
	"bytes"
	"fmt"
	"image"
	"math"
)

// Orientation values stored in game_item_versions.payload.orientation. The set
// is closed (5 buckets) — every uploaded image is snapped to its nearest
// neighbour, so the frontend can pin a fixed aspect-ratio container per bucket
// without dealing with arbitrary ratios.
const (
	OrientationLandscape4x3  = "landscape_4_3"
	OrientationLandscape16x9 = "landscape_16_9"
	OrientationSquare        = "square"
	OrientationPortrait3x4   = "portrait_3_4"
	OrientationPortrait9x16  = "portrait_9_16"
)

// orientationBuckets enumerates the targets for nearest-neighbour selection.
// Order doesn't affect correctness; argmin picks the closest distance.
var orientationBuckets = []struct {
	name  string
	ratio float64
}{
	{OrientationLandscape16x9, 16.0 / 9.0},
	{OrientationLandscape4x3, 4.0 / 3.0},
	{OrientationSquare, 1.0},
	{OrientationPortrait3x4, 3.0 / 4.0},
	{OrientationPortrait9x16, 9.0 / 16.0},
}

// DetectOrientation reads only the image header (image.DecodeConfig — does
// not allocate pixels) and snaps the file's aspect ratio to one of five
// fixed buckets. Decoders for jpeg/png are pulled in by storage/mime.go;
// webp is registered there too.
func DetectOrientation(data []byte) (string, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image header: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return "", fmt.Errorf("invalid image dimensions: %dx%d", cfg.Width, cfg.Height)
	}
	ratio := float64(cfg.Width) / float64(cfg.Height)

	best := orientationBuckets[0].name
	bestDist := math.Abs(ratio - orientationBuckets[0].ratio)
	for _, b := range orientationBuckets[1:] {
		d := math.Abs(ratio - b.ratio)
		if d < bestDist {
			best = b.name
			bestDist = d
		}
	}
	return best, nil
}
