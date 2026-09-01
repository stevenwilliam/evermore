package test

import (
	"io"
	"log/slog"
)

type slogLogger = slog.Logger

// newQuietLogger discards output so a test run's signal is the assertions, not
// a wall of request lines.
func newQuietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
