package lexbridge

// Shim is a minimal iterator over Desi NDJSON tokens.
// We'll use this to feed a parser once we add a constructor that accepts tokens.
type Shim struct {
	toks []Token
	i    int
}

// NewShim returns a new shim over the given tokens.
func NewShim(toks []Token) *Shim {
	return &Shim{toks: toks, i: 0}
}

// HasNext reports whether there is another token.
func (s *Shim) HasNext() bool {
	return s.i < len(s.toks)
}

// Peek returns the next token without advancing. If exhausted, returns a zero Token.
func (s *Shim) Peek() Token {
	if s.i < len(s.toks) {
		return s.toks[s.i]
	}
	return Token{}
}

// Next returns the next token and advances. If exhausted, returns a zero Token.
func (s *Shim) Next() Token {
	if s.i < len(s.toks) {
		t := s.toks[s.i]
		s.i++
		return t
	}
	return Token{}
}

// Reset moves the iterator to the beginning.
func (s *Shim) Reset() { s.i = 0 }
