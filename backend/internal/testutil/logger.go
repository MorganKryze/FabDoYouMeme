package testutil

import (
	"io"
	"log/slog"
)

// NewLogger returns a slog.Logger that discards all output — used by tests
// that need a logger but don't care about the messages.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
