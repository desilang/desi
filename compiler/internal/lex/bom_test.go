package lex

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/token"
)

// A .desi file saved by Notepad, or written by PowerShell's
// `Set-Content -Encoding utf8`, starts with a UTF-8 BOM. It is neither
// whitespace nor an identifier start, so it used to reach the parser as a stray
// token and the file failed at 1:1 with "unexpected token" pointing at an
// ordinary-looking first line.
func TestScannerSkipsUTF8BOM(t *testing.T) {
	const prog = "let x = 1\n"
	withBOM := append(append([]byte{}, utf8BOM...), prog...)

	plain := tokenKinds(t, []byte(prog))
	bommed := tokenKinds(t, withBOM)

	if len(plain) != len(bommed) {
		t.Fatalf("BOM changed the token count: %d without, %d with\n%v\n%v",
			len(plain), len(bommed), plain, bommed)
	}
	for i := range plain {
		if plain[i] != bommed[i] {
			t.Fatalf("token %d differs: %v without BOM, %v with", i, plain[i], bommed[i])
		}
	}

	// The first token must still be reported at column 1, so diagnostics point
	// where the reader is looking.
	s := NewScanner(withBOM)
	first := s.Next()
	if first.Line != 1 || first.Col != 1 {
		t.Fatalf("first token at %d:%d, want 1:1", first.Line, first.Col)
	}
	if len(s.Errors()) != 0 {
		t.Fatalf("BOM produced scan errors: %v", s.Errors())
	}
}

func tokenKinds(t *testing.T, src []byte) []token.Token {
	t.Helper()
	s := NewScanner(src)
	var out []token.Token
	for {
		it := s.Next()
		if it.Tok == token.EOF {
			break
		}
		out = append(out, it.Tok)
		if len(out) > 1000 {
			t.Fatal("scanner did not terminate")
		}
	}
	return out
}
