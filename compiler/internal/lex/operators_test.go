package lex

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/token"
)

func TestGreedyOperators(t *testing.T) {
	src := "2**=3 2**3 1|>f 1|2 a>=b a>b a==b"
	sc := NewScannerWithFile([]byte(src), "<ops>")
	var ops []token.Token
	for {
		it := sc.Next()
		if it.Tok == token.EOF {
			break
		}
		if token.IsOperator(it.Tok) {
			ops = append(ops, it.Tok)
		}
	}
	want := []token.Token{
		token.POW_EQ, // **=
		token.POW,    // **
		token.PIPE_GT,
		token.PIPE,
		token.GTE,
		token.GT,
		token.EQEQ,
	}
	if len(ops) != len(want) {
		t.Fatalf("got %v ops, want %v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("ops[%d]=%v want %v", i, ops[i], want[i])
		}
	}
}
