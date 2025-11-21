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

// ScanError is a non-fatal lexer error captured during scanning.
type ScanError struct {
	CodePath string // e.g., "lexer.unterminated_string" or "lexer.invalid_float_exponent"
	Message  string
	File     string
	Line     int
	Col      int
}

// Scanner converts source to Items, including layout tokens.
type Scanner struct {
	src       []byte
	i         int    // byte offset
	line, col int    // 1-based
	atBOL     bool   // at beginning of (logical) line
	indents   []int  // indent stack
	pending   []Item // queued items (Indent/Dedent etc.)

	file string
	errs []ScanError
}

// NewScanner creates a scanner for the given bytes (no filename context).
func NewScanner(src []byte) *Scanner {
	return &Scanner{
		src:     src,
		line:    1,
		col:     1,
		atBOL:   true,
		indents: []int{0},
	}
}

// NewScannerWithFile creates a scanner with file path (used for diagnostics).
func NewScannerWithFile(src []byte, filename string) *Scanner {
	s := NewScanner(src)
	s.file = filename
	return s
}

// Errors returns the collected non-fatal scan errors.
func (s *Scanner) Errors() []ScanError { return s.errs }

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

	// Handle beginning-of-line indentation & comment-only/blank lines.
	if s.atBOL {
		// Fast path: true blank line → just emit NL without changing indent.
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

		indent, isCommentLine := s.measureIndentAndComment()
		if isCommentLine {
			// Consume to end of line and the newline, then emit a single NL.
			s.consumeToEOL()
			if s.i < len(s.src) {
				r, w := utf8.DecodeRune(s.src[s.i:])
				if r == '\n' {
					s.i += w
					s.line++
					s.col = 1
				}
			}
			s.atBOL = true
			return s.emitNL()
		}

		cur := s.indents[len(s.indents)-1]
		if indent > cur {
			s.indents = append(s.indents, indent)
			return Item{Tok: token.Indent, Line: s.line, Col: 1}
		}
		if indent < cur {
			// Multiple dedents? Queue them up.
			for len(s.indents) > 0 && indent < s.indents[len(s.indents)-1] {
				s.indents = s.indents[:len(s.indents)-1]
				s.pending = append(s.pending, Item{Tok: token.Dedent, Line: s.line, Col: 1})
			}
			// Return the first one now.
			if len(s.pending) > 0 {
				it := s.pending[0]
				s.pending = s.pending[1:]
				return it
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

	// mid-line comment? keep '#' as token unless '#{' set opener; end-of-line '#' acts as comment
	if s.peekIs('#') && !s.peek2Is("#{") {
		s.consumeToEOL()
		s.atBOL = true
		return Item{Tok: token.NL, Line: s.line, Col: 1}
	}

	// Leading-dot floats: .5, .5e+2
	if s.peekIs('.') && unicode.IsDigit(s.peekRuneN(1)) {
		startCol := s.col
		start := s.i
		// consume '.'
		s.i++
		s.col++
		// must have at least one digit
		if n, ok := s.advanceDigitsSep(); n == 0 || !ok {
			s.addErr("lexer.invalid_float_fraction", "invalid float fraction", s.line, startCol)
			return Item{Tok: token.ILLEGAL, Lexeme: "invalid float fraction", Line: s.line, Col: startCol}
		}
		// optional exponent
		if s.peekIs('e') || s.peekIs('E') {
			s.i++
			s.col++
			if s.peekIs('+') || s.peekIs('-') {
				s.i++
				s.col++
			}
			if n, ok := s.advanceDigitsSep(); n == 0 || !ok {
				s.addErr("lexer.invalid_float_exponent", "invalid float exponent", s.line, startCol)
				return Item{Tok: token.ILLEGAL, Lexeme: "invalid float exponent", Line: s.line, Col: startCol}
			}
			return Item{Tok: token.FLOAT_EXP, Lexeme: string(s.src[start:s.i]), Line: s.line, Col: startCol}
		}
		return Item{Tok: token.FLOAT, Lexeme: string(s.src[start:s.i]), Line: s.line, Col: startCol}
	}

	// --- STRINGS FIRST (including f-strings) ---
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
	// --- end strings ---

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

	// numbers: int/float, supporting 0x/0b/0o, decimal fractions (incl. trailing dot), decimal exponents,
	// and hexadecimal floating-point (0x...p±...).
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
					// Hex int or hex float.
					s.i += w + w2
					s.col += 2

					nInt, okInt := s.advanceWhileSep(isHexDigit) // may be 0 if we go straight to "."
					if !okInt {
						s.addErr("lexer.invalid_hex_literal", "invalid hex literal", s.line, startCol)
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid hex literal", Line: s.line, Col: startCol}
					}

					sawFrac := false
					if s.peekIs('.') && isHexDigit(s.peekRuneN(1)) {
						sawFrac = true
						s.i++
						s.col++
						if nFrac, okFrac := s.advanceWhileSep(isHexDigit); nFrac == 0 || !okFrac {
							s.addErr("lexer.invalid_hex_fraction", "invalid hex fraction", s.line, startCol)
							return Item{Tok: token.ILLEGAL, Lexeme: "invalid hex fraction", Line: s.line, Col: startCol}
						}
					}

					// Must have at least one hex digit overall
					if nInt == 0 && !sawFrac {
						s.addErr("lexer.invalid_hex_literal", "invalid hex literal", s.line, startCol)
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid hex literal", Line: s.line, Col: startCol}
					}

					// Exponent makes it a hex float; if we saw a fraction, exponent is required.
					if s.peekIs('p') || s.peekIs('P') {
						s.i++
						s.col++
						if s.peekIs('+') || s.peekIs('-') {
							s.i++
							s.col++
						}
						if nExp, ok := s.advanceDigitsSep(); nExp == 0 || !ok {
							s.addErr("lexer.invalid_hex_float_exponent", "invalid hex float exponent", s.line, startCol)
							return Item{Tok: token.ILLEGAL, Lexeme: "invalid hex float exponent", Line: s.line, Col: startCol}
						}
						lex := string(s.src[start:s.i])
						return Item{Tok: token.FLOAT_EXP, Lexeme: lex, Line: s.line, Col: startCol}
					} else {
						if sawFrac {
							s.addErr("lexer.missing_hex_exponent", "hex float missing exponent 'p'", s.line, startCol)
							return Item{Tok: token.ILLEGAL, Lexeme: "hex float missing exponent 'p'", Line: s.line, Col: startCol}
						}
						lex := string(s.src[start:s.i])
						return Item{Tok: token.INT_HEX, Lexeme: lex, Line: s.line, Col: startCol}
					}

				case 'b', 'B':
					// 0b[01][_01]*
					s.i += w + w2
					s.col += 2
					if n, ok := s.advanceWhileSep(isBinDigit); n == 0 || !ok {
						s.addErr("lexer.invalid_binary_literal", "invalid binary literal", s.line, startCol)
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid binary literal", Line: s.line, Col: startCol}
					}
					lex := string(s.src[start:s.i])
					return Item{Tok: token.INT_BIN, Lexeme: lex, Line: s.line, Col: startCol}

				case 'o', 'O':
					// 0o[0-7][_0-7]*
					s.i += w + w2
					s.col += 2
					if n, ok := s.advanceWhileSep(isOctDigit); n == 0 || !ok {
						s.addErr("lexer.invalid_octal_literal", "invalid octal literal", s.line, startCol)
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid octal literal", Line: s.line, Col: startCol}
					}
					lex := string(s.src[start:s.i])
					return Item{Tok: token.INT_OCT, Lexeme: lex, Line: s.line, Col: startCol}
				}
			}

			// decimal: digits, optional frac (including trailing '.'), optional exponent
			dcount, ok := s.advanceDigitsSep()
			if dcount == 0 || !ok {
				s.addErr("lexer.invalid_number", "invalid number", s.line, startCol)
				return Item{Tok: token.ILLEGAL, Lexeme: "invalid number", Line: s.line, Col: startCol}
			}
			isFloat := false
			hasExp := false

			if s.peekIs('.') {
				// If the next after '.' is a digit, consume fractional digits with separators.
				if unicode.IsDigit(s.peekRuneN(1)) {
					isFloat = true
					s.i++
					s.col++
					if n, ok := s.advanceDigitsSep(); n == 0 || !ok {
						s.addErr("lexer.invalid_float_fraction", "invalid float fraction", s.line, startCol)
						return Item{Tok: token.ILLEGAL, Lexeme: "invalid float fraction", Line: s.line, Col: startCol}
					}
				} else {
					// Trailing-dot float: consume a single '.' and mark as float
					isFloat = true
					s.i++
					s.col++
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
				if n, ok := s.advanceDigitsSep(); n == 0 || !ok {
					s.addErr("lexer.invalid_float_exponent", "invalid float exponent", s.line, startCol)
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
			return Item{Tok: token.INT_DEC, Lexeme: lex, Line: s.line, Col: startCol}
		}
	}

	// operators/punctuators (greedy)
	if it, ok := s.scanOperatorOrPunct(); ok {
		return it
	}

	// unknown byte → ILLEGAL and advance one rune
	r, w := utf8.DecodeRune(s.src[s.i:])
	msg := "illegal character"
	if r != utf8.RuneError {
		msg = "illegal character: " + string(r)
	}
	s.addErr("lexer.generic_lexer_error", msg, s.line, s.col)
	it := Item{Tok: token.ILLEGAL, Lexeme: msg, Line: s.line, Col: s.col}
	s.i += w
	s.col++
	return it
}

// ---------- helpers ----------

func (s *Scanner) addErr(codePath, msg string, line, col int) {
	s.errs = append(s.errs, ScanError{
		CodePath: codePath,
		Message:  msg,
		File:     s.file,
		Line:     line,
		Col:      col,
	})
}

func (s *Scanner) emitNL() Item {
	s.atBOL = true
	return Item{Tok: token.NL, Line: s.line, Col: 1}
}

// Detect leading indentation; tabs-only policy:
// - Any leading space in indentation triggers a diagnostic.
// - We still proceed and compute indent so scanning can continue.
// Also detect comment-only lines beginning with '#' (but not '#{').
func (s *Scanner) measureIndentAndComment() (indent int, isCommentLine bool) {
	indent = 0
	hadSpace := false
	for off := s.i; off < len(s.src); {
		r, w := utf8.DecodeRune(s.src[off:])
		if r == ' ' {
			indent++
			off += w
			hadSpace = true
			continue
		}
		if r == '\t' {
			indent++
			off += w
			continue
		}
		if r == '\n' {
			// true blank line: let caller handle it (we no longer dedent on blanks)
			return 0, false
		}
		if r == '#' {
			r2 := s.peekRuneAt(off + w)
			// comment-only (ignore)
			if r2 != '{' {
				s.i = off
				s.col = indent + 1
				s.atBOL = false
				return indent, true
			}
		}
		// non-blank, non-comment line starts here
		if hadSpace {
			s.addErr("lexer.tabs_only_indentation", "spaces used for indentation; tabs required", s.line, 1)
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

// returns count of decimal digits consumed, validating '_' separators (no leading/trailing/double '_')
func (s *Scanner) advanceDigitsSep() (int, bool) {
	n := 0
	lastUnderscore := false
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if unicode.IsDigit(r) {
			s.i += w
			s.col++
			n++
			lastUnderscore = false
			continue
		}
		if r == '_' {
			// reject leading '_' or doubled '__'
			if n == 0 || lastUnderscore {
				return 0, false
			}
			s.i += w
			s.col++
			lastUnderscore = true
			continue
		}
		break
	}
	if lastUnderscore {
		return 0, false
	}
	return n, true
}

// base-specific digits with '_' separators allowed (no leading/trailing/double '_')
func (s *Scanner) advanceWhileSep(pred func(rune) bool) (int, bool) {
	n := 0
	lastUnderscore := false
	for s.i < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.i:])
		if pred(r) {
			s.i += w
			s.col++
			n++
			lastUnderscore = false
			continue
		}
		if r == '_' {
			if n == 0 || lastUnderscore {
				return 0, false
			}
			s.i += w
			s.col++
			lastUnderscore = true
			continue
		}
		break
	}
	if lastUnderscore {
		return 0, false
	}
	return n, true
}

func isHexDigit(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}
func isBinDigit(r rune) bool { return r == '0' || r == '1' }
func isOctDigit(r rune) bool { return '0' <= r && r <= '7' }

// ---------- string scanning with escape validation ----------

func (s *Scanner) scanString() Item {
	startCol := s.col
	s.i++ // consume opening "
	s.col++
	for s.i < len(s.src) {
		// close?
		if s.peekIs('"') {
			s.i++
			s.col++
			return Item{Tok: token.STR, Lexeme: "", Line: s.line, Col: startCol}
		}
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\\' {
			if !s.consumeEscape() {
				s.addErr("lexer.invalid_escape_sequence", "invalid escape sequence", s.line, s.col)
				return Item{Tok: token.ILLEGAL, Lexeme: "invalid escape sequence", Line: s.line, Col: startCol}
			}
			continue
		}
		if r == '\n' || r == utf8.RuneError {
			break
		}
		s.i += w
		s.col++
	}
	// unterminated — report via diag code
	s.addErr("lexer.unterminated_string", "string literal not closed", s.line, startCol)
	return Item{Tok: token.ILLEGAL, Lexeme: "unterminated string", Line: s.line, Col: startCol}
}

func (s *Scanner) scanFString() Item {
	startCol := s.col
	s.i += 2 // f"
	s.col += 2
	for s.i < len(s.src) {
		if s.peekIs('"') {
			s.i++
			s.col++
			return Item{Tok: token.FSTR, Lexeme: "", Line: s.line, Col: startCol}
		}
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\\' {
			if !s.consumeEscape() {
				s.addErr("lexer.invalid_escape_sequence", "invalid escape sequence", s.line, s.col)
				return Item{Tok: token.ILLEGAL, Lexeme: "invalid escape sequence", Line: s.line, Col: startCol}
			}
			continue
		}
		if r == '\n' || r == utf8.RuneError {
			break
		}
		s.i += w
		s.col++
	}
	s.addErr("lexer.unterminated_string", "f-string literal not closed", s.line, startCol)
	return Item{Tok: token.ILLEGAL, Lexeme: "unterminated f-string", Line: s.line, Col: startCol}
}

func (s *Scanner) scanLongString() Item {
	startCol := s.col
	s.i += 3 // """
	s.col += 3
	for s.i < len(s.src) {
		// close only on exact """
		if s.peek2Is(`"""`) {
			s.i += 3
			s.col += 3
			return Item{Tok: token.LONGSTR, Lexeme: "", Line: s.line, Col: startCol}
		}
		r, w := utf8.DecodeRune(s.src[s.i:])
		if r == '\\' {
			if !s.consumeEscape() {
				s.addErr("lexer.invalid_escape_sequence", "invalid escape sequence", s.line, s.col)
				return Item{Tok: token.ILLEGAL, Lexeme: "invalid escape sequence", Line: s.line, Col: startCol}
			}
			continue
		}
		if r == '\n' {
			s.i += w
			s.line++
			s.col = 1
			continue
		}
		s.i += w
		s.col++
	}
	s.addErr("lexer.unterminated_string", "long string literal not closed", s.line, startCol)
	return Item{Tok: token.ILLEGAL, Lexeme: "unterminated long string", Line: s.line, Col: startCol}
}

// validate and consume one escape sequence after the backslash.
// supports: \n \r \t \\ \" \0 \xNN \uXXXX \UXXXXXXXX (hex digits only)
func (s *Scanner) consumeEscape() bool {
	// at backslash
	if s.i >= len(s.src) {
		return false
	}
	// consume '\'
	_, w := utf8.DecodeRune(s.src[s.i:])
	s.i += w
	s.col++

	if s.i >= len(s.src) {
		return false
	}
	r, w := utf8.DecodeRune(s.src[s.i:])
	switch r {
	case 'n', 'r', 't', '\\', '"', '0', '{', '}':
		s.i += w
		s.col++
		return true
	case 'x':
		s.i += w
		s.col++
		return s.consumeHexDigits(2)
	case 'u':
		s.i += w
		s.col++
		return s.consumeHexDigits(4)
	case 'U':
		s.i += w
		s.col++
		return s.consumeHexDigits(8)
	default:
		return false
	}
}

func (s *Scanner) consumeHexDigits(n int) bool {
	for i := 0; i < n; i++ {
		if s.i >= len(s.src) {
			return false
		}
		r, w := utf8.DecodeRune(s.src[s.i:])
		if !isHexDigit(r) {
			return false
		}
		s.i += w
		s.col++
	}
	return true
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
	case "trait":
		return token.KW_trait, true
	case "impl":
		return token.KW_impl, true
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
	case "ref":
		return token.KW_ref, true
	case "inout":
		return token.KW_inout, true
	case "unsafe":
		return token.KW_unsafe, true
	case "break":
		return token.KW_break, true
	case "continue":
		return token.KW_continue, true
	default:
		return token.ILLEGAL, false
	}
}

// Convenience: ScanFile(path) → []Item  (kept for compatibility)
func ScanFile(path string) ([]Item, error) {
	items, _, err := ScanFileFull(path)
	return items, err
}

// ScanFileFull returns items and collected non-fatal scan errors.
func ScanFileFull(path string) ([]Item, []ScanError, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, bufio.NewReader(f)); err != nil {
		return nil, nil, err
	}
	sc := NewScannerWithFile(buf.Bytes(), path)
	var items []Item
	for {
		it := sc.Next()
		items = append(items, it)
		if it.Tok == token.EOF {
			break
		}
		// Note: we DO NOT break on ILLEGAL; we keep scanning to surface all errors.
	}
	return items, sc.Errors(), nil
}
