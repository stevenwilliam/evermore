// Package http is the HTTP adapter: routing, middleware, request and response
// mapping. It maps domain errors onto the one JSON error model and never lets
// a driver error through.
package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/logging"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
)

const traceHeader = "X-Request-Id"

// RequestID assigns every request a trace id and echoes it, so a user can
// quote something a log search will find.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(traceHeader)
		// Never trust a client-supplied id as-is: it lands in logs, and an
		// attacker-chosen value could forge a line or bloat an index.
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		c.Set("trace_id", id)
		c.Header(traceHeader, id)
		ctx := logging.WithTraceID(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// Logger logs one structured line per request.
func Logger(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID, _ := c.Get("trace_id")
		l := base.With("trace_id", traceID)
		c.Request = c.Request.WithContext(logging.Into(c.Request.Context(), l))

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			// sanitize.LogValue: a path with a newline in it must not be able
			// to forge a second log line.
			"path", sanitize.LogValue(c.Request.URL.Path),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", c.Writer.Size(),
		}
		switch {
		case status >= 500:
			l.Error("request failed", attrs...)
		case status >= 400:
			l.Warn("request rejected", attrs...)
		default:
			l.Info("request", attrs...)
		}
	}
}

// Recovery turns a panic into a 500 with a trace id, and logs the stack.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		l := logging.From(c.Request.Context())
		l.Error("panic recovered", "panic", fmt.Sprint(recovered))
		Fail(c, apierror.Internal(fmt.Errorf("panic: %v", recovered)))
	})
}

// SecurityHeaders sets the headers that do not depend on the response body.
//
// The CSP is deliberately strict and carries no 'unsafe-inline' for scripts.
// Google Maps needs its own origins, which are listed rather than opened up
// with a wildcard.
func SecurityHeaders(isProd bool, extraConnect ...string) gin.HandlerFunc {
	connect := append([]string{"'self'"}, extraConnect...)
	directives := []string{
		"default-src 'self'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		// Styles are in files, but inline style attributes are used for a few
		// computed values (a progress width, a map pin offset), so
		// 'unsafe-inline' is needed for style and deliberately not for script.
		"style-src 'self' 'unsafe-inline'",
		"script-src 'self' https://maps.googleapis.com",
		"img-src 'self' data: blob: https://maps.gstatic.com https://maps.googleapis.com https://*.googleapis.com",
		"font-src 'self'",
		"connect-src " + strings.Join(connect, " ") + " https://maps.googleapis.com",
		"manifest-src 'self'",
	}

	// upgrade-insecure-requests ONLY where TLS actually terminates. On a
	// plain-HTTP host the browser rewrites every subresource to https://,
	// which fails outright — caught by probing the deployed URL rather than
	// localhost, where browsers treat the origin as already trustworthy and
	// the directive is a no-op.
	if isProd {
		directives = append(directives, "upgrade-insecure-requests")
	}
	csp := strings.Join(directives, "; ")

	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(self), camera=(), microphone=(), payment=(), interest-cohort=()")
		if isProd {
			// Only over real TLS. Sending HSTS from a plain-HTTP dev box would
			// pin a browser to https for a host that does not serve it.
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// CORS allows exactly the configured origins. There is no wildcard and no
// origin reflection: reflecting Origin with credentials enabled is the same
// as having no policy at all.
func CORS(allowed []string) gin.HandlerFunc {
	allowSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowSet[strings.TrimRight(o, "/")] = true
	}
	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if origin != "" && allowSet[origin] {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, "+traceHeader+", Idempotency-Key")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// BodyLimit caps request bodies. Without it a single request can exhaust
// memory, and the multipart parser is happy to try.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// Fail writes an error using the one JSON error model.
func Fail(c *gin.Context, err error) {
	e := apierror.From(err)
	if id, ok := c.Get("trace_id"); ok {
		if s, ok := id.(string); ok {
			e.TraceID = s
		}
	}
	// The cause is logged, never serialised — a pgx error names tables,
	// columns and constraints (CLAUDE.md §4).
	if cause := errors.Unwrap(e); cause != nil {
		logging.From(c.Request.Context()).Error("request error",
			"code", string(e.Code), "cause", sanitize.LogValue(cause.Error()))
	}
	c.AbortWithStatusJSON(e.Status, gin.H{"error": e})
}

// OK writes a successful JSON response.
func OK(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}
