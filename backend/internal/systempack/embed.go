// backend/internal/systempack/embed.go
package systempack

import "embed"

// DemoPackFS is the filesystem of bundled demo-pack assets.
// Files here are baked into the binary at build time; the operator updates
// the pack by adding or replacing files and rebuilding the image.
//
//go:embed demo-pack/*
var DemoPackFS embed.FS
