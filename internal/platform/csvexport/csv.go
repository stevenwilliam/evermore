// Package csvexport writes the pipe-delimited exports every report ships.
//
// CLAUDE.md §7: the delimiter is a pipe, never a comma, because Indonesian
// addresses, names and notes contain commas constantly. It is still a real
// RFC 4180 CSV — quoting, doubled quotes, CRLF — with '|' as the separator,
// and every cell is guarded against formula injection.
package csvexport

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
)

// Delimiter is the pipe. It is a constant rather than an option because a
// report that exported commas would silently corrupt on the first address
// containing one.
const Delimiter = '|'

// Writer emits a pipe-delimited CSV with formula-injection guards.
type Writer struct {
	w    *csv.Writer
	cols int
	rows int
}

// New returns a Writer. It emits a UTF-8 BOM, because Excel on Windows
// otherwise reads the file as the system codepage and mangles every Indonesian
// name with a non-ASCII character in it.
func New(w io.Writer) (*Writer, error) {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return nil, err
	}
	cw := csv.NewWriter(w)
	cw.Comma = Delimiter
	cw.UseCRLF = true // RFC 4180
	return &Writer{w: cw}, nil
}

// Header writes the header row and fixes the column count for the file.
func (w *Writer) Header(cols ...string) error {
	w.cols = len(cols)
	return w.w.Write(cols)
}

// Row writes one record, guarding every cell.
func (w *Writer) Row(cells ...string) error {
	// A row with the wrong number of columns produces a file that opens
	// misaligned, which is the kind of thing nobody notices until a finance
	// report is wrong. Refuse it here instead.
	if w.cols > 0 && len(cells) != w.cols {
		return fmt.Errorf("csvexport: row %d has %d cells, header declared %d", w.rows+1, len(cells), w.cols)
	}
	guarded := make([]string, len(cells))
	for i, c := range cells {
		guarded[i] = sanitize.CSVCell(c)
	}
	w.rows++
	return w.w.Write(guarded)
}

// Flush writes any buffered data and reports the first error encountered.
func (w *Writer) Flush() error {
	w.w.Flush()
	return w.w.Error()
}

// Rows is how many data rows were written, for the "exported N rows" message.
func (w *Writer) Rows() int { return w.rows }

// --- cell formatters, so every report renders a value the same way ---

// IDR renders a rupiah amount as plain digits: no separators, no currency
// symbol, no decimals. A spreadsheet can format it; a spreadsheet cannot
// reliably un-format "Rp 480.148".
func IDR(v money.IDR) string { return strconv.FormatInt(int64(v), 10) }

// Int renders an integer.
func Int(v int) string { return strconv.Itoa(v) }

// Date renders a business date as ISO-8601 in the operating timezone.
func Date(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2006-01-02")
}

// DateTime renders a timestamp in the operating timezone. Storage is UTC;
// exports are read by people in Jakarta, so the conversion is explicit here
// rather than left to the server's zone (CLAUDE.md §4).
func DateTime(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

// Bool renders a boolean the way the Indonesian UI reads it.
func Bool(v bool) string {
	if v {
		return "Ya"
	}
	return "Tidak"
}
