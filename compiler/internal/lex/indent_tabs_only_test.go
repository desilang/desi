package lex

import "testing"

func TestTabsOnly_RejectsSpacesIndent(t *testing.T) {
	src := "def f():\n  let a = 1\n\tlet b = 2\n" // second line has spaces at BOL
	_, errs := toks(src)
	found := false
	for _, e := range errs {
		if e.CodePath == "lexer.tabs_only_indentation" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected lexer.tabs_only_indentation, got %v", errs)
	}
}

func TestTabsOnly_AllTabsOK(t *testing.T) {
	src := "def f():\n\tlet a = 1\n\t\tlet b = 2\n"
	_, errs := toks(src)
	for _, e := range errs {
		if e.CodePath == "lexer.tabs_only_indentation" {
			t.Fatalf("unexpected tabs-only indentation error: %v", e)
		}
	}
}
