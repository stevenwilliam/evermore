// Package apierror is the one JSON error model this service speaks.
//
// CLAUDE.md §4: errors are typed, there is one error model, and driver errors
// never reach a client. A pq/pgx error string can name a table, a column and a
// constraint, which is a free schema disclosure to anyone who can provoke it.
package apierror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable error code. Clients switch on this, not
// on the message, so a message can be reworded without breaking anyone.
type Code string

const (
	CodeValidation       Code = "VALIDATION_FAILED"
	CodeUnauthenticated  Code = "UNAUTHENTICATED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConflict         Code = "CONFLICT"
	CodeRateLimited      Code = "RATE_LIMITED"
	CodeInternal         Code = "INTERNAL"
	CodePayloadTooLarge  Code = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMedia Code = "UNSUPPORTED_MEDIA_TYPE"

	// Domain codes, surfaced verbatim so the UI can react precisely.
	CodePriceNotConfigured Code = "PRICE_NOT_CONFIGURED"
	CodeNotServiceable     Code = "ADDRESS_NOT_SERVICEABLE"
	CodeCapacityFull       Code = "CAPACITY_FULL"
	CodeInsufficientCredit Code = "INSUFFICIENT_CREDIT"
	CodePastCutOff         Code = "PAST_CUTOFF"
	CodeIllegalTransition  Code = "ILLEGAL_TRANSITION"
	CodePackageExpired     Code = "PACKAGE_EXPIRED"
	CodeMenuNotPublished   Code = "MENU_NOT_PUBLISHED"
	CodeDuplicateRequest   Code = "DUPLICATE_REQUEST"
)

// FieldError is one field-level validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is the single error type crossing the HTTP boundary.
type Error struct {
	Status  int          `json:"-"`
	Code    Code         `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
	// TraceID lets a user quote something a log search can find, without the
	// response carrying any internal detail.
	TraceID string `json:"trace_id,omitempty"`

	// cause is never serialised. It is what gets logged.
	cause error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the cause to errors.Is/As for logging and testing, never to
// the response writer.
func (e *Error) Unwrap() error { return e.cause }

// WithCause attaches the underlying error for logging.
func (e *Error) WithCause(err error) *Error {
	c := *e
	c.cause = err
	return &c
}

// WithField adds a field-level detail.
func (e *Error) WithField(field, code, msg string) *Error {
	c := *e
	c.Fields = append(append([]FieldError(nil), c.Fields...), FieldError{field, code, msg})
	return &c
}

func New(status int, code Code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

// Constructors for the common cases. Messages are Indonesian, the default
// locale; the handler layer localises where a catalogue key exists.
func Validation(msg string) *Error {
	if msg == "" {
		msg = "Data yang dikirim tidak valid."
	}
	return New(http.StatusBadRequest, CodeValidation, msg)
}
func Unauthenticated(msg string) *Error {
	if msg == "" {
		msg = "Silakan masuk terlebih dahulu."
	}
	return New(http.StatusUnauthorized, CodeUnauthenticated, msg)
}
func Forbidden(msg string) *Error {
	if msg == "" {
		msg = "Anda tidak punya akses ke sumber daya ini."
	}
	return New(http.StatusForbidden, CodeForbidden, msg)
}
func NotFound(msg string) *Error {
	if msg == "" {
		msg = "Data tidak ditemukan."
	}
	return New(http.StatusNotFound, CodeNotFound, msg)
}
func Conflict(code Code, msg string) *Error { return New(http.StatusConflict, code, msg) }
func RateLimited(msg string) *Error {
	return New(http.StatusTooManyRequests, CodeRateLimited, msg)
}

// Internal is the only constructor that deliberately discards detail. The
// caller attaches the cause with WithCause; the client sees nothing of it.
func Internal(cause error) *Error {
	return (&Error{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternal,
		Message: "Terjadi kesalahan pada sistem. Silakan coba lagi.",
	}).WithCause(cause)
}

// From maps any error to an *Error. An error that is not already one becomes
// an Internal, which is what stops a driver error reaching a client by
// default rather than by remembering to wrap it.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}
