package main

import (
	"os"

	"github.com/desilang/desi/compiler/internal/lexer"
	"github.com/desilang/desi/compiler/internal/term"
)

/* ---------- lex (Go lexer) ---------- */

func cmdLexDirect(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		term.Eprintf("read %s: %v\n", path, err)
		return 1
	}
	lx := lexer.New(string(data))
	for {
		t := lx.Next()
		if t.Kind == lexer.TokEOF {
			term.Printf("%d:%d  %s\n", t.Line, t.Col, t.Kind)
			break
		}
		lex := t.Lex
		if len(lex) > 40 {
			lex = lex[:37] + "..."
		}
		if lex == "" {
			term.Printf("%d:%d  %-8s\n", t.Line, t.Col, t.Kind)
		} else {
			term.Printf("%d:%d  %-8s  %q\n", t.Line, t.Col, t.Kind, lex)
		}
	}
	return 0
}
