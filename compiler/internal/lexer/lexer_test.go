package lexer

import "testing"

func TestStubEOF(t *testing.T) {
	l := New("")
	tok := l.Next()
	if tok.Kind != TokEOF {
		t.Fatalf("expected EOF, got %v", tok.Kind)
	}
}
