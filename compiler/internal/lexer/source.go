package lexer

// Source is a minimal token source the parser can consume.
// Any implementation only needs to yield successive tokens via Next().
type Source interface {
	Next() Token
}

// goSource adapts the existing Go lexer to the Source interface.
type goSource struct {
	lx *Lexer
}

// NewSource returns a Source backed by the existing Go lexer for the input string.
func NewSource(src string) Source {
	return &goSource{lx: New(src)}
}

// Next satisfies Source by delegating to the underlying Go lexer.
func (s *goSource) Next() Token {
	return s.lx.Next()
}
