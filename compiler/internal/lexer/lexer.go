package lexer

import (
	"unicode"
)

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

// handle beginning-of-line: compute indentation and queue INDENT/DEDENT/skip blanks.
func (lx *Lexer) handleBOL() {
	for lx.bol {
		// EOF: unwind any remaining indents
		if lx.atEOF() {
			for len(lx.indents) > 1 {
				lx.indents = lx.indents[:len(lx.indents)-1]
				lx.enqueue(lx.make(TokDedent, "", lx.line, lx.col))
			}
			lx.bol = false
			return
		}

		// Count indentation (spaces/tabs) but don't consume newline yet
		width := 0
		for {
			ch, ok := lx.peek()
			if !ok {
				break
			}
			if ch == ' ' {
				width++
				lx.advance()
				continue
			}
			if ch == '\t' {
				width += 4 // Stage-0: TAB = 4 spaces
				lx.advance()
				continue
			}
			break
		}

		// Blank or comment-only line? Consume to newline and continue at BOL.
		if ch, ok := lx.peek(); !ok {
			// EOF after spaces: just unwind in next loop
		} else if ch == '\n' {
			lx.advance() // eat newline
			// keep bol=true; skip emitting NEWLINE for blank lines
			continue
		} else if ch == '#' {
			// consume comment to end-of-line
			for {
				ch, ok := lx.peek()
				if !ok || ch == '\n' {
					break
				}
				lx.advance()
			}
			if lx.match('\n') {
				// comment-only line: skip NEWLINE
				continue
			}
			// fallthrough if EOF
		}

		// Compare indentation with top of stack
		top := lx.indents[len(lx.indents)-1]
		if width > top {
			lx.indents = append(lx.indents, width)
			lx.enqueue(lx.make(TokIndent, "", lx.line, lx.col))
		} else if width < top {
			for width < top && len(lx.indents) > 1 {
				lx.indents = lx.indents[:len(lx.indents)-1]
				top = lx.indents[len(lx.indents)-1]
				lx.enqueue(lx.make(TokDedent, "", lx.line, lx.col))
			}
			// If width != top here, it's a malformed indent; Stage-0: ignore extra for now.
		}
		lx.bol = false
		// We leave lx.i at first non-space char to be lexed by Next()
		if len(lx.pending) > 0 {
			return
		}
		// Otherwise we proceed to lex the token on this line.
		return
	}
}

// Next returns the next token. It never panics on user input.
func (lx *Lexer) Next() Token {
	// Emit any queued tokens first
	if n := len(lx.pending); n > 0 {
		t := lx.pending[0]
		lx.pending = lx.pending[1:]
		return t
	}

	// Handle indentation if at beginning of a logical line
	if lx.bol {
		lx.handleBOL()
		if n := len(lx.pending); n > 0 {
			t := lx.pending[0]
			lx.pending = lx.pending[1:]
			return t
		}
	}

	// EOF: unwind remaining indents, then emit EOF
	if lx.atEOF() {
		if !lx.eofEmitted {
			// Safety: ensure indent stack unwound
			for len(lx.indents) > 1 {
				lx.indents = lx.indents[:len(lx.indents)-1]
				return lx.make(TokDedent, "", lx.line, lx.col)
			}
			lx.eofEmitted = true
		}
		return lx.make(TokEOF, "", lx.line, lx.col)
	}

	// Skip mid-line spaces/tabs
	for {
		ch, ok := lx.peek()
		if !ok {
			break
		}
		if ch == ' ' || ch == '\t' {
			lx.advance()
			continue
		}
		break
	}

	startLine, startCol := lx.line, lx.col+1

	// Newline terminates a statement, emit NEWLINE and go to BOL
	if ch, ok := lx.peek(); ok && ch == '\n' {
		lx.advance()
		lx.bol = true
		return lx.make(TokNewline, "", startLine, startCol)
	}

	// Comment mid-line: consume to EOL, then emit NEWLINE
	if ch, ok := lx.peek(); ok && ch == '#' {
		for {
			ch, ok := lx.peek()
			if !ok || ch == '\n' {
				break
			}
			lx.advance()
		}
		if lx.match('\n') {
			lx.bol = true
			return lx.make(TokNewline, "", startLine, startCol)
		}
		// EOF after comment
		return lx.make(TokEOF, "", lx.line, lx.col)
	}

	// Identifiers / keywords
	if ch, ok := lx.peek(); ok && (isIdentStart(ch)) {
		lex := lx.scanIdent()
		if kind, ok := keywordKind(lex); ok {
			return lx.make(kind, lex, startLine, startCol)
		}
		return lx.make(TokIdent, lex, startLine, startCol)
	}

	// Numbers (decimal, 0x..., 0b...)
	if ch, ok := lx.peek(); ok && unicode.IsDigit(ch) {
		lex := lx.scanNumber()
		return lx.make(TokInt, lex, startLine, startCol)
	}

	// Strings (simple "..." with basic escapes)
	if ch, ok := lx.peek(); ok && ch == '"' {
		lex := lx.scanString()
		return lx.make(TokStr, lex, startLine, startCol)
	}

	// Multi-char operators first
	if lx.match(':') {
		if lx.match('=') {
			return lx.make(TokAssign, ":=", startLine, startCol)
		}
		return lx.make(TokColon, ":", startLine, startCol)
	}
	if lx.match('-') {
		if lx.match('>') {
			return lx.make(TokArrow, "->", startLine, startCol)
		}
		return lx.make(TokMinus, "-", startLine, startCol)
	}
	if lx.match('=') {
		if lx.match('=') {
			return lx.make(TokEqEq, "==", startLine, startCol)
		}
		return lx.make(TokEq, "=", startLine, startCol)
	}
	if lx.match('!') {
		if lx.match('=') {
			return lx.make(TokNe, "!=", startLine, startCol)
		}
		return lx.make(TokBang, "!", startLine, startCol)
	}
	if lx.match('<') {
		if lx.match('=') {
			return lx.make(TokLe, "<=", startLine, startCol)
		}
		return lx.make(TokLt, "<", startLine, startCol)
	}
	if lx.match('>') {
		if lx.match('=') {
			return lx.make(TokGe, ">=", startLine, startCol)
		}
		return lx.make(TokGt, ">", startLine, startCol)
	}
	if lx.match('|') {
		if lx.match('>') {
			return lx.make(TokPipe, "|>", startLine, startCol)
		}
		// unknown '|' — Stage-0: return as Dot? Better: just treat as TokDot? No, return '.' would be wrong; fall-through
		return lx.make(TokPipe, "|", startLine, startCol)
	}

	// Single-char punctuation
	if lx.match('+') {
		return lx.make(TokPlus, "+", startLine, startCol)
	}
	if lx.match('*') {
		return lx.make(TokStar, "*", startLine, startCol)
	}
	if lx.match('/') {
		return lx.make(TokSlash, "/", startLine, startCol)
	}
	if lx.match('%') {
		return lx.make(TokPercent, "%", startLine, startCol)
	}
	if lx.match('(') {
		return lx.make(TokLParen, "(", startLine, startCol)
	}
	if lx.match(')') {
		return lx.make(TokRParen, ")", startLine, startCol)
	}
	if lx.match('[') {
		return lx.make(TokLBrack, "[", startLine, startCol)
	}
	if lx.match(']') {
		return lx.make(TokRBrack, "]", startLine, startCol)
	}
	if lx.match('.') {
		return lx.make(TokDot, ".", startLine, startCol)
	}
	if lx.match(',') {
		return lx.make(TokComma, ",", startLine, startCol)
	}

	// Unknown character: skip it and continue (Stage-0 lenient)
	lx.advance()
	return lx.Next()
}

// ----- scanning helpers -----

func isIdentStart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r)
}
func isIdentPart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (lx *Lexer) scanIdent() string {
	start := lx.i
	for {
		r, ok := lx.peek()
		if !ok || !isIdentPart(r) {
			break
		}
		lx.advance()
	}
	return string(lx.src[start:lx.i])
}

func (lx *Lexer) scanNumber() string {
	start := lx.i
	// 0x / 0b prefixes
	if ch, ok := lx.peek(); ok && ch == '0' {
		lx.advance()
		if ch2, ok2 := lx.peek(); ok2 && (ch2 == 'x' || ch2 == 'X') {
			lx.advance()
			for {
				r, ok := lx.peek()
				if !ok || !(unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
					break
				}
				lx.advance()
			}
			return string(lx.src[start:lx.i])
		}
		if ch2, ok2 := lx.peek(); ok2 && (ch2 == 'b' || ch2 == 'B') {
			lx.advance()
			for {
				r, ok := lx.peek()
				if !ok || !(r == '0' || r == '1') {
					break
				}
				lx.advance()
			}
			return string(lx.src[start:lx.i])
		}
		// fallthrough to decimal after single '0'
	}
	for {
		r, ok := lx.peek()
		if !ok || !unicode.IsDigit(r) {
			break
		}
		lx.advance()
	}
	return string(lx.src[start:lx.i])
}

func (lx *Lexer) scanString() string {
	start := lx.i
	lx.advance() // consume opening "
	for {
		r, ok := lx.peek()
		if !ok {
			break
		}
		if r == '\\' {
			lx.advance() // backslash
			_, _ = lx.advance()
			continue
		}
		if r == '"' {
			lx.advance()
			break
		}
		// allow newlines to terminate strings? Stage-0: stop at newline too
		if r == '\n' {
			break
		}
		lx.advance()
	}
	return string(lx.src[start:lx.i])
}

// keywordKind maps identifiers to keyword tokens.
func keywordKind(s string) (TokKind, bool) {
	switch s {
	case "let":
		return TokLet, true
	case "mut":
		return TokMut, true
	case "def":
		return TokDef, true
	case "return":
		return TokReturn, true
	case "if":
		return TokIf, true
	case "elif":
		return TokElif, true
	case "else":
		return TokElse, true
	case "while":
		return TokWhile, true
	case "for":
		return TokFor, true
	case "in":
		return TokIn, true
	case "match":
		return TokMatch, true
	case "struct":
		return TokStruct, true
	case "enum":
		return TokEnum, true
	case "package":
		return TokPackage, true
	case "import":
		return TokImport, true
	case "as":
		return TokAs, true
	case "true":
		return TokTrue, true
	case "false":
		return TokFalse, true
	case "and":
		return TokAnd, true
	case "or":
		return TokOr, true
	case "not":
		return TokNot, true
	default:
		return 0, false
	}
}
