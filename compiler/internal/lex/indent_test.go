package lex

import "testing"

func TestMixedIndentationWarns(t *testing.T) {
	src := "x\n \t1\n" // second line mixes space + tab before '1'
	_, errs := toks(src)
	found := false
	for _, e := range errs {
		if e.CodePath == "lexer.mixed_indentation" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected lexer.mixed_indentation diagnostic, got %v", errs)
	}
}
