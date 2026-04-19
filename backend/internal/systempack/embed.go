// backend/internal/systempack/embed.go
package systempack

import "embed"

// DemoPackFS is the filesystem of bundled demo-pack assets.
// Files here are baked into the binary at build time; the operator updates
// the pack by adding or replacing files and rebuilding the image.
//
//go:embed demo-pack/*
var DemoPackFS embed.FS

// DemoTextPackFS holds the bundled text demo pack — currently a single
// items.json with `[{"name": "...", "text": "..."}, ...]` entries. Same
// rebuild-the-image-to-update model as DemoPackFS.
//
//go:embed demo-text-pack/*
var DemoTextPackFS embed.FS
