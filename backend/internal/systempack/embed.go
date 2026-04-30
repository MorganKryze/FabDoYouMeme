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

// DemoTextPackFRFS is the French-language sibling of DemoTextPackFS. Text is
// inherently locale-bound (captions/prompts are authored in one language), so
// each locale ships its own pack rather than a machine-translated copy.
//
//go:embed demo-text-pack-fr/*
var DemoTextPackFRFS embed.FS

// DemoPromptPackFS is the bundled English fill-in-the-blank prompt pack.
// Each items.json entry is a `{name, prefix, suffix}` record (payload v4)
// where prefix/suffix straddle the blank in the rendered sentence.
//
//go:embed demo-prompt-pack/*
var DemoPromptPackFS embed.FS

// DemoPromptPackFRFS is the French-language prompt pack — same shape as
// DemoPromptPackFS, locale-bound content.
//
//go:embed demo-prompt-pack-fr/*
var DemoPromptPackFRFS embed.FS

// DemoFillerPackFS is the bundled English filler pack — short noun-phrase
// cards (payload v3, `{text}`) dealt as the showdown hand for prompt-showdown.
//
//go:embed demo-filler-pack/*
var DemoFillerPackFS embed.FS

// DemoFillerPackFRFS is the French-language filler pack.
//
//go:embed demo-filler-pack-fr/*
var DemoFillerPackFRFS embed.FS
