package lexer_test

import (
	"testing"

	lx "github.com/desilang/desi/compiler/internal/lexer"
)

func TestStubEOF(t *testing.T) {
	l := lx.New("")
	tok := l.Next()
	if tok.Kind != lx.TokEOF {
		t.Fatalf("expected EOF, got %v", tok.Kind)
	}
}
