package lex

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/token"
)

func toks(src string) ([]Item, []ScanError) {
	sc := NewScannerWithFile([]byte(src), "<mem>")
	var items []Item
	for {
		it := sc.Next()
		items = append(items, it)
		if it.Tok == token.EOF {
			break
		}
	}
	return items, sc.Errors()
}

func TestHexFloatGood(t *testing.T) {
	items, errs := toks("0x1p4 0x1.fp3 0x.8p+2")
	if got := len(errs); got != 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
	var count int
	for _, it := range items {
		if it.Tok == token.FLOAT_EXP {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("want 3 FLOAT_EXP, got %d; items=%v", count, items)
	}
}

func TestHexFloatMissingP(t *testing.T) {
	items, errs := toks("0x1.f 0x_p1 0x1p+_2")
	if len(errs) != 3 {
		t.Fatalf("want 3 errs, got %d (%v)", len(errs), errs)
	}
	// Ensure we didn't stop after first ILLEGAL
	if items[len(items)-1].Tok != token.EOF {
		t.Fatalf("did not reach EOF")
	}
}

func TestSeparatorsAndDots(t *testing.T) {
	_, errs := toks(strings.Join([]string{
		".5", "2.", "1_000_000", "1.2_34e+5", "0b1010_0101", "0o7_55", "0xDEAD_BEEF",
	}, " "))
	if got := len(errs); got != 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
}
