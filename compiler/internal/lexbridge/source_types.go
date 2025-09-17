package lexbridge

import "github.com/desilang/desi/compiler/internal/lexer"

// desiSource replays a pre-mapped slice of Go tokens (lexer.TokKind)
// produced from the Desi NDJSON stream.
type desiSource struct {
	toks []lexer.Token
	i    int
}

func (s *desiSource) Next() lexer.Token {
	if s.i >= len(s.toks) {
		// Safety: once drained, always return EOF
		return lexer.Token{Kind: lexer.TokEOF}
	}
	t := s.toks[s.i]
	s.i++
	return t
}
