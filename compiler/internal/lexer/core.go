package lexer

// Lexer scans source into tokens, producing NEWLINE/INDENT/DEDENT like Python.
// It treats TAB as 4 spaces for indentation. Stage-0 keeps it simple.
type Lexer struct {
	src []rune
	i   int

	line int
	col  int

	bol        bool    // beginning-of-line: next non-space decides indentation
	indents    []int   // stack of indent widths; starts with 0
	pending    []Token // queued tokens (e.g., INDENT/DEDENT/NEWLINE)
	eofEmitted bool
}

func New(src string) *Lexer {
	return &Lexer{
		src:     []rune(src),
		line:    1,
		col:     0,
		bol:     true,
		indents: []int{0},
	}
}

func (lx *Lexer) enqueue(t Token) { lx.pending = append(lx.pending, t) }

func (lx *Lexer) make(kind TokKind, lex string, line, col int) Token {
	return Token{Kind: kind, Lex: lex, Line: line, Col: col}
}

func (lx *Lexer) peek() (rune, bool) {
	if lx.i >= len(lx.src) {
		return 0, false
	}
	return lx.src[lx.i], true
}

func (lx *Lexer) advance() (rune, bool) {
	ch, ok := lx.peek()
	if !ok {
		return 0, false
	}
	lx.i++
	if ch == '\n' {
		lx.line++
		lx.col = 0
	} else {
		lx.col++
	}
	return ch, true
}

func (lx *Lexer) match(expect rune) bool {
	ch, ok := lx.peek()
	if ok && ch == expect {
		lx.advance()
		return true
	}
	return false
}

func (lx *Lexer) atEOF() bool { return lx.i >= len(lx.src) }
