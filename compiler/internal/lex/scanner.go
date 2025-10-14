package lex

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/desilang/desi/compiler/internal/token"
)

// Item is a scanned token with basic position & lexeme (for dumps/debug).
type Item struct {
	Tok    token.Token
	Lexeme string
	Line   int
	Col    int
}

// Scanner converts source to Items, including layout tokens.
type Scanner struct {
	src       []byte
	i         int    // byte offset
	line, col int    // 1-based
	atBOL     bool   // at beginning of (logical) line
	indents   []int  // indent stack
	pending   []Item // queued items (Indent/Dedent etc.)
}

// NewScanner creates a scanner for the given bytes.
func NewScanner(src []byte) *Scanner {
	return &Scanner{
		src:     src,
		line:    1,
		col:     1,
		atBOL:   true,
		indents: []int{0},
	}
}

// Next returns the next token Item (EOF at end).
func (s *Scanner) Next() Item {
	// drain queued (Indent/Dedent/NL/etc.)
	if n := len(s.pending); n > 0 {
		it := s.pending[0]
		s.pending = s.pending[1:]
		return it
	}

	// EOF → flush pending dedents once.
	if s.i >= len(s.src) {
		for len(s.indents) > 1 {
			s.indents = s.indents[:len(s.indents)-1]
			return Item{Tok: token.Dedent, Line: s.line, Col: s.col}
		}
		return Item{Tok: token.EOF, Line: s.line, Col: s.col}
	}

	// Handle beginning-of-line indentation & comment-only lines.
	if s.atBOL {
		indent, isCommentLine := s.measureIndentAndComment()
		if isCommentLine {
			s.consumeToEOL()
			s.atBOL = true
			return s.emitNL()
		}
		cur := s.indents[len(s.indents)-1]
		if indent > cur {
			s.indents = append(s.indents, indent)
			return Item{Tok: token.Indent, Line: s.line, Col: 1}
		}
		if indent < cur {
			for len(s.indents) > 0 && indent < s.indents[len(s.indents)-1] {
				s.indents = s.indents[:len(s.indents)-1]
				return Item{Tok: token.Dedent, Line: s.line, Col: 1}
			}
		}
	}

	// Skip whitespace (not newlines)
	for {
		if s.i >= len(s.src) {
			break
		}
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\r' {
			s.i += w
			continue
		}
		if r == ' ' || r == '\t' {
			s.i += w
			s.col++
			continue
		}
		break
	}

	// newline?
	if s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\n' {
			s.i += w
			s.line++
			s.col = 1
			s.atBOL = true
			return Item{Tok: token.NL, Line: s.line - 1, Col: 1}
		}
	}

	// mid-line comment? keep '#' as token unless '#{' set literal; end-of-line '#' acts as comment
	if s.peekIs('#') && !s.peek2Is("#{") {
		s.consumeToEOL()
		s.atBOL = true
		return Item{Tok: token.NL, Line: s.line, Col: 1}
	}

	// identifier / keyword
	if s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '_' || unicode.IsLetter(r) {
			startCol := s.col
			start := s.i
			s.i += w
			s.col++
			for s.i < len(s.src) {
				r2, w2 := utf8.DecodeRune(s.src[s.i:])
				if r2 == '_' || unicode.IsLetter(r2) || unicode.IsDigit(r2) {
					s.i += w2
					s.col++
					continue
				}
				break
			}
			lex := string(s.src[start:s.i])
			if kwTok, ok := keywordToken(lex); ok {
				return Item{Tok: kwTok, Lexeme: lex, Line: s.line, Col: startCol}
			}
			return Item{Tok: token.IDENT, Lexeme: lex, Line: s.line, Col: startCol}
		}
	}

	// numbers: int/float, supporting 0x/0b/0o and decimal exponents
	if s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if unicode.IsDigit(r) {
			startCol := s.col
			start := s.i

			// base prefixes when starting with '0'
			if r == '0' && s.i+w < len(s.src) {
				r2, w2 := utf8.DecodeRune(s.src[s.i+w:])
				switch r2 {
				case 'x', 'X':
					// 0x[0-9a-fA-F]+
					s.i += w + w2
					s.col += 2
					if s.advanceWhile(isHexDigit) == 0 {
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid hex literal", Line: s.line, Col: startCol}
					}
					lex := string(s.src[start:s.i])
					return Item{Tok: token.INT_HEX, Lexeme: lex, Line: s.line, Col: startCol}
				case 'b', 'B':
					// 0b[01]+
					s.i += w + w2
					s.col += 2
					if s.advanceWhile(isBinDigit) == 0 {
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid binary literal", Line: s.line, Col: startCol}
					}
					lex := string(s.src[start:s.i])
					return Item{Tok: token.INT_BIN, Lexeme: lex, Line: s.line, Col: startCol}
				case 'o', 'O':
					// 0o[0-7]+
					s.i += w + w2
					s.col += 2
					if s.advanceWhile(isOctDigit) == 0 {
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid octal literal", Line: s.line, Col: startCol}
					}
					lex := string(s.src[start:s.i])
					return Item{Tok: token.INT_OCT, Lexeme: lex, Line: s.line, Col: startCol}
				}
			}

			// decimal: digits, optional frac, optional exponent
			dcount := s.advanceDigits()
			isFloat := false
			hasExp := false

			if s.peekIs('.') && unicode.IsDigit(s.peekRuneN(1)) {
				isFloat = true
				s.i++
				s.col++
				if s.advanceDigits() == 0 {
					return Item{Tok: token.ILLEGAL, Lexeme: "invalid float fraction", Line: s.line, Col: startCol}
				}
			}
			if s.peekIs('e') || s.peekIs('E') {
				// exponent part
				isFloat = true
				hasExp = true
				s.i++
				s.col++
				if s.peekIs('+') || s.peekIs('-') {
					s.i++
					s.col++
				}
				if s.advanceDigits() == 0 {
					return Item{Tok: token.ILLEGAL, Lexeme: "invalid float exponent", Line: s.line, Col: startCol}
				}
			}

			lex := string(s.src[start:s.i])
			if isFloat {
				if hasExp {
					return Item{Tok: token.FLOAT_EXP, Lexeme: lex, Line: s.line, Col: startCol}
				}
				return Item{Tok: token.FLOAT, Lexeme: lex, Line: s.line, Col: startCol}
			}
			// plain decimal int
			if dcount == 0 {
				return Item{Tok: token.ILLEGAL, Lexeme: "invalid number", Line: s.line, Col: startCol}
			}
			return Item{Tok: token.INT_DEC, Lexeme: lex, Line: s.line, Col: startCol}
		}
	}

	// strings: """...""", f"...", "..."
	if s.peek2Is(`"""`) {
		return s.scanLongString()
	}
	if s.peekIs('f') && s.peekRuneN(1) == '"' {
		return s.scanFString()
	}
	if s.peekIs('"') {
		return s.scanString()
	}

	// operators/punctuators (greedy)
	if it, ok := s.scanOperatorOrPunct(); ok {
		return it
	}

	// unknown byte → ILLEGAL and advance one rune
	r, w := utf8.DecodeRune(s.src[s.i:])
	it := Item{Tok: token.ILLEGAL, Lexeme: string(r), Line: s.line, Col: s.col}
	s.i += w
	s.col++
	return it
}

// ---------- helpers ----------

func (s *Scanner) emitNL() Item {
	s.atBOL = true
	return Item{Tok: token.NL, Line: s.line, Col: 1}
}

func (s *Scanner) measureIndentAndComment() (indent int, isCommentLine bool) {
	indent = 0
	for off := s.i; off < len(s.src); {
		r, w := utf8.DecodeRune(s.src[off:])
		if r == ' ' || r == '\t' {
			indent++
			off += w
			continue
		}
		if r == '\n' {
			return 0, false
		}
		if r == '#' {
			r2 := s.peekRuneAt(off + w)
			if r2 != '{' {
				s.i = off
				s.col = indent + 1
				s.atBOL = false
				return indent, true
			}
		}
		s.i = off
		s.col = indent + 1
		s.atBOL = false
		return indent, false
	}
	return indent, false
}

func (s *Scanner) consumeToEOL() {
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\n' {
			return
		}
		s.i += w
		s.col++
	}
}

// returns count of digits consumed
func (s *Scanner) advanceDigits() int {
	n := 0
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if unicode.IsDigit(r) {
			s.i += w
			s.col++
			n++
		} else {
			return n
		}
	}
	return n
}

func isHexDigit(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}
func isBinDigit(r rune) bool { return r == '0' || r == '1' }
func isOctDigit(r rune) bool { return '0' <= r && r <= '7' }

// returns count of digits consumed for a predicate
func (s *Scanner) advanceWhile(pred func(rune) bool) int {
	n := 0
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if pred(r) {
			s.i += w
			s.col++
			n++
		} else {
			return n
		}
	}
	return n
}

func (s *Scanner) scanString() Item {
	startCol := s.col
	s.i++ // consume opening "
	s.col++
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\\' { // escape next
			s.i += w
			s.col++
			if s.i < len(s.src) {
				_, w2 := utf8.DecodeRune(s.src[s.i:])
				s.i += w2
				s.col++
			}
			continue
		}
		if r == '"' {
			s.i += w
			s.col++
			return Item{Tok: token.STR, Lexeme: "", Line: s.line, Col: startCol}
		}
		if r == '\n' || r == utf8.RuneError {
			break
		}
		s.i += w
		s.col++
	}
	return Item{Tok: token.ILLEGAL, Lexeme: "unterminated string", Line: s.line, Col: startCol}
}

func (s *Scanner) scanFString() Item {
	startCol := s.col
	s.i += 2 // f"
	s.col += 2
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\\' {
			s.i += w
			s.col++
			if s.i < len(s.src) {
				_, w2 := utf8.DecodeRune(s.src[s.i:])
				s.i += w2
				s.col++
			}
			continue
		}
		if r == '"' {
			s.i += w
			s.col++
			return Item{Tok: token.FSTR, Lexeme: "", Line: s.line, Col: startCol}
		}
		if r == '\n' || r == utf8.RuneError {
			break
		}
		s.i += w
		s.col++
	}
	return Item{Tok: token.ILLEGAL, Lexeme: "unterminated f-string", Line: s.line, Col: startCol}
}

func (s *Scanner) scanLongString() Item {
	startCol := s.col
	s.i += 3 // """
	s.col += 3
	for s.i < len(s.src) {
		if s.peek2Is(`"""`) {
			s.i += 3
			s.col += 3
			return Item{Tok: token.LONGSTR, Lexeme: "", Line: s.line, Col: startCol}
		}
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\n' {
			s.i += w
			s.line++
			s.col = 1
			continue
		}
		s.i += w
		s.col++
	}
	return Item{Tok: token.ILLEGAL, Lexeme: "unterminated long string", Line: s.line, Col: startCol}
}

func (s *Scanner) scanOperatorOrPunct() (Item, bool) {
	lits := token.OperatorLits()
	for _, lit := range lits {
		if s.hasPrefix(lit) {
			tok, _ := token.OperatorByLit(lit)
			it := Item{Tok: tok, Lexeme: lit, Line: s.line, Col: s.col}
			s.i += len(lit)
			s.col += runeCount(lit)
			return it, true
		}
	}
	return Item{}, false
}

func (s *Scanner) hasPrefix(x string) bool {
	if s.i+len(x) > len(s.src) {
		return false
	}
	return bytes.Equal(s.src[s.i:s.i+len(x)], []byte(x))
}

func (s *Scanner) peekIs(ch rune) bool {
	if s.i >= len(s.src) {
		return false
	}
	r, _ := utf8.DecodeRune(s.src[s.i:])
	return r == ch
}

func (s *Scanner) peek2Is(s2 string) bool { return s.hasPrefix(s2) }

func (s *Scanner) peekRuneN(n int) rune {
	off := s.i
	for n > 0 && off < len(s.src) {
		_, w := utf8.DecodeRune(s.src[off:])
		off += w
		n--
	}
	if off >= len(s.src) {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRune(s.src[off:])
	return r
}

func (s *Scanner) peekRuneAt(off int) rune {
	if off >= len(s.src) {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRune(s.src[off:])
	return r
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func keywordToken(lex string) (token.Token, bool) {
	if !token.IsKeyword(lex) {
		return token.ILLEGAL, false
	}
	switch lex {
	case "import":
		return token.KW_import, true
	case "from":
		return token.KW_from, true
	case "as":
		return token.KW_as, true
	case "pub":
		return token.KW_pub, true
	case "def":
		return token.KW_def, true
	case "async":
		return token.KW_async, true
	case "class":
		return token.KW_class, true
	case "struct":
		return token.KW_struct, true
	case "enum":
		return token.KW_enum, true
	case "type":
		return token.KW_type, true
	case "let":
		return token.KW_let, true
	case "mut":
		return token.KW_mut, true
	case "return":
		return token.KW_return, true
	case "if":
		return token.KW_if, true
	case "elif":
		return token.KW_elif, true
	case "else":
		return token.KW_else, true
	case "while":
		return token.KW_while, true
	case "for":
		return token.KW_for, true
	case "in":
		return token.KW_in, true
	case "using":
		return token.KW_using, true
	case "defer":
		return token.KW_defer, true
	case "match":
		return token.KW_match, true
	case "select":
		return token.KW_select, true
	case "await":
		return token.KW_await, true
	case "true":
		return token.KW_true, true
	case "false":
		return token.KW_false, true
	case "none":
		return token.KW_none, true
	case "and":
		return token.KW_and, true
	case "or":
		return token.KW_or, true
	case "not":
		return token.KW_not, true
	default:
		return token.ILLEGAL, false
	}
}

// Convenience: ScanFile(path) → []Item
func ScanFile(path string) ([]Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, bufio.NewReader(f)); err != nil {
		return nil, err
	}
	sc := NewScanner(buf.Bytes())
	var items []Item
	for {
		it := sc.Next()
		items = append(items, it)
		if it.Tok == token.EOF || it.Tok == token.ILLEGAL {
			break
		}
	}
	return items, nil
}
