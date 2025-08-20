package lexer

// Lexer is a placeholder for Stage-0 scaffolding.
// We'll implement indentation-aware scanning next.
type Lexer struct {
	src       []rune
	i         int
	line, col int
}

func New(src string) *Lexer { return &Lexer{src: []rune(src), line: 1, col: 0} }

// Next returns the next token. Stage-0 stub returns EOF so the pipeline links.
func (lx *Lexer) Next() Token {
	return Token{Kind: TokEOF, Line: lx.line, Col: lx.col}
}
