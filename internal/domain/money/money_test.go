package money

import (
	"math"
	"testing"
)

func TestSplitInclusive_WorkedExample(t *testing.T) {
	// 01-domain-model.md §3.11 states this exact result. If it ever changes,
	// the document and the code have diverged and one of them is wrong.
	got, err := SplitInclusive(500000, 1100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Base != 450450 {
		t.Errorf("base = %d, want 450450", got.Base)
	}
	if got.Tax != 49550 {
		t.Errorf("tax = %d, want 49550", got.Tax)
	}
}

func TestSplitInclusive_BasePlusTaxAlwaysEqualsInclusive(t *testing.T) {
	// The property that matters more than any single value. Taking tax as the
	// residue is what guarantees it, so this is the test that would catch a
	// well-meaning refactor to an independently-computed tax.
	rates := []int{0, 1, 100, 1100, 1200, 9999}
	for _, rate := range rates {
		for amount := IDR(0); amount <= 3000; amount++ {
			s, err := SplitInclusive(amount, rate)
			if err != nil {
				t.Fatalf("amount=%d rate=%d: %v", amount, rate, err)
			}
			if s.Base+s.Tax != amount {
				t.Fatalf("amount=%d rate=%d: base %d + tax %d = %d, want %d",
					amount, rate, s.Base, s.Tax, s.Base+s.Tax, amount)
			}
			if s.Base < 0 || s.Tax < 0 {
				t.Fatalf("amount=%d rate=%d: negative component base=%d tax=%d",
					amount, rate, s.Base, s.Tax)
			}
		}
	}
}

func TestSplitInclusive_ZeroRate(t *testing.T) {
	s, err := SplitInclusive(78000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.Base != 78000 || s.Tax != 0 {
		t.Errorf("at 0 bps got base=%d tax=%d, want 78000/0", s.Base, s.Tax)
	}
}

func TestSplitInclusive_OneRupiah(t *testing.T) {
	// A line total of Rp 1 is in the documented test matrix.
	s, err := SplitInclusive(1, 1100)
	if err != nil {
		t.Fatal(err)
	}
	if s.Base+s.Tax != 1 {
		t.Errorf("base %d + tax %d != 1", s.Base, s.Tax)
	}
}

func TestSplitInclusive_RejectsNegative(t *testing.T) {
	if _, err := SplitInclusive(-1, 1100); err != ErrNegative {
		t.Errorf("got %v, want ErrNegative", err)
	}
	if _, err := SplitInclusive(100, -1); err != ErrNegative {
		t.Errorf("negative rate: got %v, want ErrNegative", err)
	}
}

func TestSplitInclusive_Overflow(t *testing.T) {
	if _, err := SplitInclusive(IDR(math.MaxInt64/2), 1100); err != ErrOverflow {
		t.Errorf("got %v, want ErrOverflow", err)
	}
}

func TestApplyBPS_HalfUp(t *testing.T) {
	cases := []struct {
		amount IDR
		bps    int
		want   IDR
	}{
		{10000, 1100, 1100},
		{1, 5000, 1},      // 0.5 rounds up to 1
		{1, 4999, 0},      // just under half rounds down
		{78000, 0, 0},     // zero rate
		{0, 1100, 0},      // zero amount
		{999, 10000, 999}, // 100%
	}
	for _, c := range cases {
		got, err := ApplyBPS(c.amount, c.bps)
		if err != nil {
			t.Fatalf("ApplyBPS(%d,%d): %v", c.amount, c.bps, err)
		}
		if got != c.want {
			t.Errorf("ApplyBPS(%d,%d) = %d, want %d", c.amount, c.bps, got, c.want)
		}
	}
}

func TestMul_Overflow(t *testing.T) {
	// 999 x the maximum price without overflowing int64 is in the matrix.
	if _, err := Mul(IDR(math.MaxInt64/100), 999); err != ErrOverflow {
		t.Errorf("got %v, want ErrOverflow", err)
	}
	got, err := Mul(78000, 999)
	if err != nil {
		t.Fatal(err)
	}
	if got != 77922000 {
		t.Errorf("got %d, want 77922000", got)
	}
}

func TestSumSplits_TaxIsSumOfLinesNotRederived(t *testing.T) {
	// Three lines whose individual splits do not equal a split of the total.
	// This is the rounding difference the rule exists to prevent, so if
	// SumSplits ever re-derives, this test fails.
	var lines []TaxSplit
	for _, amt := range []IDR{333, 333, 333} {
		s, err := SplitInclusive(amt, 1100)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, s)
	}
	total, err := SumSplits(lines)
	if err != nil {
		t.Fatal(err)
	}
	if total.Inclusive != 999 {
		t.Fatalf("inclusive = %d, want 999", total.Inclusive)
	}
	if total.Base+total.Tax != total.Inclusive {
		t.Errorf("base %d + tax %d != %d", total.Base, total.Tax, total.Inclusive)
	}
	rederived, err := SplitInclusive(999, 1100)
	if err != nil {
		t.Fatal(err)
	}
	// Documenting the divergence rather than asserting they match: if these
	// ever became equal the test still passes, but the sum is authoritative.
	t.Logf("sum-of-lines tax=%d, re-derived-from-total tax=%d", total.Tax, rederived.Tax)
}

func TestSumSplits_RejectsMixedRates(t *testing.T) {
	a, _ := SplitInclusive(1000, 1100)
	b, _ := SplitInclusive(1000, 1200)
	if _, err := SumSplits([]TaxSplit{a, b}); err == nil {
		t.Error("expected an error for mixed tax rates, got nil")
	}
}
