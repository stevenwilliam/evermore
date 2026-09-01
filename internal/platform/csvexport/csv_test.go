package csvexport

import (
	"bytes"
	"strings"
	"testing"
)

func write(t *testing.T, fn func(*Writer)) string {
	t.Helper()
	var buf bytes.Buffer
	w, err := New(&buf)
	if err != nil {
		t.Fatal(err)
	}
	fn(w)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestDelimiterIsAPipe(t *testing.T) {
	out := write(t, func(w *Writer) {
		_ = w.Header("nama", "alamat")
		_ = w.Row("Sinta Prameswari", "Jl. Wijaya IX No. 12, Petogogan, Kebayoran Baru")
	})
	if !strings.Contains(out, "nama|alamat") {
		t.Errorf("header is not pipe-delimited: %q", out)
	}
	// The whole point: an address full of commas stays in ONE cell and is not
	// quoted, because the comma is not the separator.
	if !strings.Contains(out, "Sinta Prameswari|Jl. Wijaya IX No. 12, Petogogan, Kebayoran Baru") {
		t.Errorf("the comma-laden address did not survive as one cell: %q", out)
	}
}

func TestFormulaInjectionIsNeutralised(t *testing.T) {
	out := write(t, func(w *Writer) {
		_ = w.Header("catatan")
		_ = w.Row("=cmd|'/c calc'!A1")
	})
	if !strings.Contains(out, "'=cmd") {
		t.Errorf("the formula was not neutralised: %q", out)
	}
}

func TestPipeInsideACellIsQuoted(t *testing.T) {
	// Still a real RFC 4180 CSV: a cell containing the delimiter is quoted.
	out := write(t, func(w *Writer) {
		_ = w.Header("catatan")
		_ = w.Row("kiri|kanan")
	})
	if !strings.Contains(out, `"kiri|kanan"`) {
		t.Errorf("a pipe inside a cell must be quoted: %q", out)
	}
}

func TestBOMIsEmitted(t *testing.T) {
	out := write(t, func(w *Writer) { _ = w.Header("a") })
	if !strings.HasPrefix(out, "\xEF\xBB\xBF") {
		t.Error("no UTF-8 BOM — Excel on Windows will mangle non-ASCII names")
	}
}

func TestCRLFLineEndings(t *testing.T) {
	out := write(t, func(w *Writer) {
		_ = w.Header("a")
		_ = w.Row("b")
	})
	if !strings.Contains(out, "a\r\n") {
		t.Errorf("expected CRLF per RFC 4180: %q", out)
	}
}

func TestRowWidthMismatchIsRefused(t *testing.T) {
	var buf bytes.Buffer
	w, _ := New(&buf)
	_ = w.Header("a", "b", "c")
	if err := w.Row("only", "two"); err == nil {
		t.Error("a row with the wrong column count must be refused, not written misaligned")
	}
}

func TestIDRIsPlainDigits(t *testing.T) {
	if got := IDR(480148); got != "480148" {
		t.Errorf("got %q, want plain digits with no separators", got)
	}
}
