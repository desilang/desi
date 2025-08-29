package lexbridge

import "testing"

func sampleTokens() []Token {
	return []Token{
		{Kind: "KW", Text: "def", Line: 1, Col: 1},
		{Kind: "IDENT", Text: "main", Line: 1, Col: 5},
		{Kind: "LPAREN", Text: "(", Line: 1, Col: 9},
		{Kind: "RPAREN", Text: ")", Line: 1, Col: 10},
		{Kind: "EOF", Text: "", Line: 1, Col: 11},
	}
}

func TestShimBasicIteration(t *testing.T) {
	toks := sampleTokens()
	s := NewShim(toks)

	// initial state
	if !s.HasNext() {
		t.Fatalf("HasNext=false at start, want true")
	}
	if p := s.Peek(); p.Kind != "KW" || p.Text != "def" || p.Line != 1 || p.Col != 1 {
		t.Fatalf("Peek mismatch at start: %#v", p)
	}

	// consume first
	if n := s.Next(); n.Kind != "KW" || n.Text != "def" {
		t.Fatalf("Next[0] mismatch: %#v", n)
	}

	// now should point at IDENT
	if !s.HasNext() {
		t.Fatalf("HasNext=false after first Next, want true")
	}
	if p := s.Peek(); p.Kind != "IDENT" || p.Text != "main" {
		t.Fatalf("Peek after Next mismatch: %#v", p)
	}

	// consume rest
	sequence := []string{"IDENT", "LPAREN", "RPAREN", "EOF"}
	for i, want := range sequence {
		n := s.Next()
		if n.Kind != want {
			t.Fatalf("Next[%d] kind=%s, want %s", i+1, n.Kind, want)
		}
	}

	// exhausted
	if s.HasNext() {
		t.Fatalf("HasNext=true after consuming all, want false")
	}
	zero := s.Next()
	if zero.Kind != "" || zero.Text != "" || zero.Line != 0 || zero.Col != 0 {
		t.Fatalf("Next at EOF should return zero token, got %#v", zero)
	}
}

func TestShimReset(t *testing.T) {
	s := NewShim(sampleTokens())

	_ = s.Next() // advance once
	s.Reset()

	if !s.HasNext() {
		t.Fatalf("HasNext=false after Reset, want true")
	}
	if p := s.Peek(); p.Kind != "KW" || p.Text != "def" {
		t.Fatalf("Peek after Reset mismatch: %#v", p)
	}
}
