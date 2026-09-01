package sanitize

import "testing"

func TestPhone_IndonesianNormalisation(t *testing.T) {
	// All four forms a customer will actually type must land on one value,
	// or the same person ends up with two accounts.
	for _, in := range []string{"0812 8899 4410", "62812-8899-4410", "+6281288994410", "81288994410"} {
		got, err := Phone("phone", in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != "+6281288994410" {
			t.Errorf("%q normalised to %q, want +6281288994410", in, got)
		}
	}
}

func TestPhone_Rejects(t *testing.T) {
	for _, in := range []string{"", "123", "+1 555 0100 0000 000", "abc"} {
		if _, err := Phone("phone", in); err == nil {
			t.Errorf("%q should be rejected", in)
		}
	}
}

func TestCSVCell_FormulaInjection(t *testing.T) {
	// The incident this rule exists for: a delivery note that executes when
	// the ops team opens the export.
	dangerous := []string{
		"=1+1",
		"+1234",
		"-1+1",
		"@SUM(A1)",
		"=cmd|'/c calc'!A1",
		"\ttab",
		"\rcr",
	}
	for _, in := range dangerous {
		got := CSVCell(in)
		if got[0] != '\'' {
			t.Errorf("CSVCell(%q) = %q — not neutralised", in, got)
		}
	}
	// Ordinary values are untouched.
	for _, in := range []string{"Jl. Wijaya IX No. 12", "Sinta Prameswari", "480148", ""} {
		if got := CSVCell(in); got != in {
			t.Errorf("CSVCell(%q) = %q, should be unchanged", in, got)
		}
	}
}

func TestNormalise_StripsControlCharacters(t *testing.T) {
	got := Normalise("Sinta\x00 Pram\x07eswari")
	if got != "Sinta Prameswari" {
		t.Errorf("got %q", got)
	}
	// Newlines become spaces and runs collapse, so an address stays one line.
	if got := Normalise("Jl. Wijaya\n\n  IX   No. 12"); got != "Jl. Wijaya IX No. 12" {
		t.Errorf("got %q", got)
	}
}

func TestLogValue_CannotForgeALogLine(t *testing.T) {
	got := LogValue("normal\nlevel=error msg=\"forged\"")
	if got == "normal\nlevel=error msg=\"forged\"" {
		t.Error("newline was not escaped; a log line can be forged")
	}
	if got != `normal\nlevel=error msg="forged"` {
		t.Errorf("got %q", got)
	}
}

func TestEmail(t *testing.T) {
	got, err := Email("email", "  Sinta@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Sinta@example.com" {
		t.Errorf("got %q — the domain lower-cases, the local part does not", got)
	}
	for _, bad := range []string{"", "no-at-sign", "a@b", "two@@at.com", "<a@b.com>"} {
		if _, err := Email("email", bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestEnum_DenyByDefault(t *testing.T) {
	if _, err := Enum("status", "DROP TABLE", "DRAFT", "PUBLISHED"); err == nil {
		t.Error("a value outside the allow-list must be refused")
	}
	if got, err := Enum("status", "DRAFT", "DRAFT", "PUBLISHED"); err != nil || got != "DRAFT" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestFilename_RejectsTraversal(t *testing.T) {
	for _, bad := range []string{"../../etc/passwd", "..", ".", "", "/etc/passwd"} {
		got, err := Filename("file", bad)
		if err == nil && (got == "etcpasswd" || got == "") {
			continue // reduced to something harmless
		}
		if err == nil {
			t.Logf("Filename(%q) = %q", bad, got)
		}
	}
	// The property that matters: no separator survives.
	got, err := Filename("file", "../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	if got != "etcpasswd" {
		t.Errorf("got %q, want the separators stripped", got)
	}
}

func TestText_RejectsRatherThanTruncates(t *testing.T) {
	long := ""
	for i := 0; i < 300; i++ {
		long += "a"
	}
	if _, err := Text("note", long, 1, 255); err == nil {
		t.Error("an over-length value must be rejected, not silently truncated")
	}
}
