package lex

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/token"
)

func TestKeywords_M6_ref_inout(t *testing.T) {
	items, _ := toks("ref inout x")
	if items[0].Tok != token.KW_ref {
		t.Fatalf("ref should scan as KW_ref, got %v", items[0].Tok)
	}
	if items[1].Tok != token.KW_inout {
		t.Fatalf("inout should scan as KW_inout, got %v", items[1].Tok)
	}
	if items[2].Tok != token.IDENT {
		t.Fatalf("x should be IDENT, got %v", items[2].Tok)
	}
}
