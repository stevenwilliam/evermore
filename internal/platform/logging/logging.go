// Package logging is the structured logger. Nothing secret is ever logged;
// values arriving from users go through sanitize.LogValue so a newline cannot
// forge an entry.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey int

const (
	ctxKeyLogger ctxKey = iota
	ctxKeyTraceID
)

// New builds the process logger. Development gets text for readability;
// anything else gets JSON so a log shipper can parse it.
func New(level, env string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lv}
	var h slog.Handler
	if env == "development" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

// Into attaches a logger to a context.
func Into(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, l)
}

// From retrieves the request logger, falling back to the default so a missing
// logger degrades to unstructured output rather than a nil dereference.
func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// WithTraceID stores the request's trace id.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyTraceID, id)
}

// TraceID reads the request's trace id, empty if there is none.
func TraceID(ctx context.Context) string {
	s, _ := ctx.Value(ctxKeyTraceID).(string)
	return s
}
