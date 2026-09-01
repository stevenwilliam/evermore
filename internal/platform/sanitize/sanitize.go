// Package sanitize validates and normalises every value arriving from outside.
//
// CLAUDE.md §4: every input is validated and sanitized on both sides. The
// frontend validates for feedback; this package exists because the frontend
// can be bypassed with curl. Two rules run through all of it:
//
//   - Reject, never silently repair. A value that is wrong comes back as an
//     error the user can act on, not as a quietly different value.
//   - Normalise (trim, Unicode, case-fold) BEFORE validating, so that a rule
//     cannot be evaded with a combining character or a full-width digit.
package sanitize

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Error is a field-level validation failure.
type Error struct {
	Field   string
	Code    string
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

func fail(field, code, msg string) *Error { return &Error{Field: field, Code: code, Message: msg} }

// Normalise applies NFC, trims surrounding whitespace, and collapses internal
// runs of whitespace to a single space. Control characters are removed
// entirely — they have no legitimate place in a name or an address line and
// they are how a log line gets forged.
func Normalise(s string) string {
	s = norm.NFC.String(s)
	s = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// Text normalises and length-checks a free-text field.
func Text(field, value string, minLen, maxLen int) (string, error) {
	v := Normalise(value)
	n := len([]rune(v))
	if n < minLen {
		return "", fail(field, "TOO_SHORT", fmt.Sprintf("Minimal %d karakter.", minLen))
	}
	if n > maxLen {
		return "", fail(field, "TOO_LONG", fmt.Sprintf("Maksimal %d karakter.", maxLen))
	}
	return v, nil
}

// Required is Text with a minimum of one character.
func Required(field, value string, maxLen int) (string, error) {
	return Text(field, value, 1, maxLen)
}

// Email normalises and validates an address. The local part keeps its case
// (it is technically case-sensitive); the domain is lower-cased, which is
// where case genuinely does not matter.
func Email(field, value string) (string, error) {
	v := strings.TrimSpace(norm.NFC.String(value))
	if v == "" {
		return "", fail(field, "REQUIRED", "Email wajib diisi.")
	}
	if len(v) > 254 {
		return "", fail(field, "TOO_LONG", "Email terlalu panjang.")
	}
	addr, err := mail.ParseAddress(v)
	if err != nil || addr.Address != v {
		return "", fail(field, "INVALID", "Format email tidak valid.")
	}
	at := strings.LastIndex(v, "@")
	// RFC 5322 permits a dotless domain, so mail.ParseAddress accepts "a@b".
	// The app_user_email_shape CHECK does not, and a value that passes here
	// only to be refused by the database becomes a 500 instead of a field
	// error. The two rules are kept deliberately identical.
	domain := v[at+1:]
	if i := strings.Index(domain, "."); i <= 0 || i == len(domain)-1 {
		return "", fail(field, "INVALID", "Format email tidak valid.")
	}
	return v[:at] + strings.ToLower(v[at:]), nil
}

var nonDigit = regexp.MustCompile(`[^0-9+]`)

// Phone normalises an Indonesian mobile number to +62 form.
//
// 0812..., 62812..., +62812... and 812... all mean the same number, and a
// customer will type all four. Storing one form is what makes a lookup work
// and what stops two accounts existing for one person.
func Phone(field, value string) (string, error) {
	v := nonDigit.ReplaceAllString(strings.TrimSpace(value), "")
	if v == "" {
		return "", fail(field, "REQUIRED", "Nomor telepon wajib diisi.")
	}
	switch {
	case strings.HasPrefix(v, "+62"):
		v = v[3:]
	case strings.HasPrefix(v, "62"):
		v = v[2:]
	case strings.HasPrefix(v, "0"):
		v = v[1:]
	}
	if strings.ContainsRune(v, '+') {
		return "", fail(field, "INVALID", "Nomor telepon tidak valid.")
	}
	if len(v) < 8 || len(v) > 13 {
		return "", fail(field, "INVALID", "Nomor telepon Indonesia terdiri dari 9–14 digit.")
	}
	return "+62" + v, nil
}

// Enum checks a value against an allow-list. Deny by default: anything not
// explicitly listed is refused.
func Enum(field, value string, allowed ...string) (string, error) {
	v := strings.TrimSpace(value)
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", fail(field, "NOT_ALLOWED",
		fmt.Sprintf("Nilai harus salah satu dari: %s.", strings.Join(allowed, ", ")))
}

var slugRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Slug validates a URL slug.
func Slug(field, value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	if !slugRE.MatchString(v) {
		return "", fail(field, "INVALID", "Slug hanya boleh huruf kecil, angka dan tanda hubung.")
	}
	if len(v) > 120 {
		return "", fail(field, "TOO_LONG", "Slug maksimal 120 karakter.")
	}
	return v, nil
}

// IntRange validates a bounded integer.
func IntRange(field string, value, min, max int) (int, error) {
	if value < min || value > max {
		return 0, fail(field, "OUT_OF_RANGE", fmt.Sprintf("Nilai harus antara %d dan %d.", min, max))
	}
	return value, nil
}

// LatLng validates a coordinate pair.
func LatLng(field string, lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return fail(field, "OUT_OF_RANGE", "Lintang harus antara -90 dan 90.")
	}
	if lng < -180 || lng > 180 {
		return fail(field, "OUT_OF_RANGE", "Bujur harus antara -180 dan 180.")
	}
	return nil
}

// CSVCell guards a value on the way OUT, into a spreadsheet.
//
// Every report in this system exports to Excel, and a cell beginning =, +, -
// or @ is a formula: a customer's delivery note becomes code execution on
// whoever opens the export. Prefixing an apostrophe is the OWASP-recommended
// neutralisation and Excel renders it as the plain text.
//
// Tab and carriage return are included because Excel treats a leading one as
// the start of a formula in some locales.
func CSVCell(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}

// LogValue makes a value safe to put in a log line: newlines and carriage
// returns become escapes, so a user-supplied string cannot forge a log entry.
func LogValue(v string) string {
	r := strings.NewReplacer("\n", "\\n", "\r", "\\r")
	return r.Replace(v)
}

// Filename reduces a name to something safe to store and serve. It refuses
// rather than repairs when nothing usable is left.
func Filename(field, value string) (string, error) {
	v := Normalise(value)
	v = strings.ReplaceAll(v, "/", "")
	v = strings.ReplaceAll(v, "\\", "")
	v = strings.TrimLeft(v, ".")
	if v == "" || v == "." || v == ".." {
		return "", fail(field, "INVALID", "Nama berkas tidak valid.")
	}
	if len(v) > 255 {
		return "", fail(field, "TOO_LONG", "Nama berkas terlalu panjang.")
	}
	return v, nil
}
