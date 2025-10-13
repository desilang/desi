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
		// Skip comment-only lines (no NL/indent changes)
		if isCommentLine {
			s.consumeToEOL()
			s.atBOL = true
			return s.emitNL()
		}
		// Indent/Dedent changes
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
		// fallthrough to scan content on this line
	}

	// Skip whitespace (not newlines)
	for {
		if s.i >= len(s.src) {
			break
		}
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\r' { // normalize CRLF
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

	// comment starting mid-line? Only treat as comment if first non-space was '#'.
	// Here mid-line '#' is a HASH punctuator, unless it starts '#{' (set literal).
	if s.peekIs('#') && !s.peek2Is("#{") {
		s.consumeToEOL()
		s.atBOL = true
		return Item{Tok: token.NL, Line: s.line, Col: 1} // end-of-line comment → NL
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

	// number (int/float minimal)
	if s.i < len(s.src) {
		r, _ := utf8.DecodeRune(s.src[s.i:])
		if unicode.IsDigit(r) {
			startCol := s.col
			start := s.i
			isFloat := false
			s.advanceDigits()
			if s.peekIs('.') && unicode.IsDigit(s.peekRuneN(1)) {
				isFloat = true
				s.i++
				s.col++
				s.advanceDigits()
			}
			lex := string(s.src[start:s.i])
			if isFloat {
				return Item{Tok: token.FLOAT, Lexeme: lex, Line: s.line, Col: startCol}
			}
			return Item{Tok: token.INT, Lexeme: lex, Line: s.line, Col: startCol}
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

	// operators/punctuators (greedy, using token.OperatorLits)
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
	// measure spaces/tabs
	for off := s.i; off < len(s.src); {
		r, w := utf8.DecodeRune(s.src[off:])
		if r == ' ' || r == '\t' {
			indent++
			off += w
			continue
		}
		// blank line?
		if r == '\n' {
			return 0, false
		}
		// comment-only? starts with '#' but not '#{'
		if r == '#' {
			// lookahead one more rune
			r2 := s.peekRuneAt(off + w)
			if r2 != '{' {
				// consume leading whitespace portion now
				s.i = off
				s.col = indent + 1
				s.atBOL = false
				return indent, true
			}
		}
		// not space/tab: set scanner at first non-space
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
			// Do not consume newline here—Next() will see it and emit NL.
			return
		}
		s.i += w
		s.col++
	}
}

func (s *Scanner) advanceDigits() {
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if unicode.IsDigit(r) {
			s.i += w
			s.col++
		} else {
			return
		}
	}
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
	// consume leading f"
	s.i += 2
	s.col += 2
	// naive scan until closing "
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
	// consume opening """
	s.i += 3
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
	// greedy: longest first
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

func (s *Scanner) peek2Is(s2 string) bool {
	return s.hasPrefix(s2)
}

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
	// Reuse token.IsKeyword to decide and then map to the actual KW_* token
	if !token.IsKeyword(lex) {
		return token.ILLEGAL, false
	}
	// map once via a small switch, mirroring keywords.go (fast path)
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
