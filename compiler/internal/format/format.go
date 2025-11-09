package format

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lex"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/token"
)

// FormatModule exists for API parity, but without original source we can’t
// safely preserve literal lexemes (esp. strings) or comments. Use FormatBytes.
func FormatModule(_ *ast.Module) ([]byte, error) {
	return nil, fmt.Errorf("format: FormatModule unavailable without original source; use FormatBytes")
}

// FormatBytes parses to validate syntax (so we can bail on bad code) and then
// re-scans tokens to rewrite ONLY whitespace/trivia deterministically.
// Idempotent by construction. Tabs-only at BOL. No trailing spaces. Trailing NL.
func FormatBytes(src []byte) ([]byte, []diag.Diagnostic) {
	if _, diags := parse.ParseFile("<stdin>", src); len(diags) > 0 {
		return nil, diags
	}
	out := rewrite(src)
	return out, nil
}

// ---- internal writer ----

type writer struct {
	buf            bytes.Buffer
	indentTabs     int
	atBOL          bool // beginning of logical line
	prevTok        token.Token
	prevPrevTok    token.Token
	lineHasContent bool

	parenDepth int
	brackDepth int
	braceDepth int

	// lineKind tracks what kind of content we emitted since the last NL.
	// 0=unknown, 1=code (idents/ops/nums/etc), 2=stringOnly (only STR/FSTR/LONGSTR so far)
	lineKind int

	// Track the physical source line of the last token that counts as "code".
	lastCodeLine int
}

const (
	_lineUnknown    = 0
	_lineCode       = 1
	_lineStringOnly = 2
)

func newWriter() *writer { return &writer{atBOL: true, lineKind: _lineUnknown} }

func (w *writer) writeIndent() {
	for i := 0; i < w.indentTabs; i++ {
		_ = w.buf.WriteByte('\t')
	}
	w.atBOL = false
}

func (w *writer) nl() {
	_ = w.buf.WriteByte('\n')
	w.atBOL = true
	w.lineHasContent = false
	w.lineKind = _lineUnknown
	w.lastCodeLine = 0
}

func (w *writer) space() {
	if w.atBOL {
		return
	}
	b := w.buf.Bytes()
	if len(b) == 0 || b[len(b)-1] == ' ' {
		return
	}
	_ = w.buf.WriteByte(' ')
}

func (w *writer) tok(s string) {
	if w.atBOL {
		w.writeIndent()
	}
	_, _ = w.buf.WriteString(s)
	w.lineHasContent = true
	// lineKind is set by caller based on token type.
}

// ---- token rewriting ----

func rewrite(src []byte) []byte {
	sc := lex.NewScannerWithFile(src, "<stdin>")
	w := newWriter()

	// Build 1-based line-start table for (line,col)->byte offset mapping.
	lineStarts := make([]int, 0, 64)
	lineStarts = append(lineStarts, 0) // dummy (unused)
	lineStarts = append(lineStarts, 0) // line 1 starts at 0
	for i := 0; i < len(src); {
		r, sz := utf8.DecodeRune(src[i:])
		if r == '\n' {
			lineStarts = append(lineStarts, i+sz)
		}
		i += sz
	}
	byteIndex := func(line, col int) int {
		if line < 1 {
			return 0
		}
		if line >= len(lineStarts) {
			return len(src)
		}
		off := lineStarts[line]
		// columns are rune-based; advance (col-1) runes
		n := col - 1
		for n > 0 && off < len(src) {
			_, sz := utf8.DecodeRune(src[off:])
			off += sz
			n--
		}
		return off
	}

	// Helper: get raw line (without trailing '\n' or '\r') for a 1-based line number.
	getLineBytes := func(line int) []byte {
		if line < 1 {
			return nil
		}
		start := 0
		if line < len(lineStarts) {
			start = lineStarts[line]
		} else {
			// beyond known lines -> last known start
			start = lineStarts[len(lineStarts)-1]
		}
		end := len(src)
		// find next newline after start
		if line+1 < len(lineStarts) {
			end = lineStarts[line+1]
		} else {
			// no precomputed next line; scan to next '\n' if any
			if idx := bytes.IndexByte(src[start:], '\n'); idx >= 0 {
				end = start + idx + 1
			}
		}
		// trim trailing '\n'
		if end > start && src[end-1] == '\n' {
			end--
		}
		// trim trailing '\r' (CRLF safety)
		if end > start && src[end-1] == '\r' {
			end--
		}
		if start < 0 {
			start = 0
		}
		if end < start {
			end = start
		}
		return src[start:end]
	}

	var it lex.Item
	var cur token.Token
	var next lex.Item
	var haveNext bool

	peekItem := func() lex.Item {
		if haveNext {
			return next
		}
		next = sc.Next()
		haveNext = true
		return next
	}
	advance := func() lex.Item {
		if haveNext {
			haveNext = false
			return next
		}
		return sc.Next()
	}

	for {
		it = advance()
		cur = it.Tok
		switch cur {
		case token.EOF:
			if !w.atBOL {
				w.nl()
			}
			return w.buf.Bytes()

		case token.NL:
			// Attach trailing EOL comment (if any) ONLY for lines with code.
			hadEOLComment := false
			if w.lineHasContent && w.lineKind == _lineCode && w.lastCodeLine > 0 {
				raw := getLineBytes(w.lastCodeLine)
				// extra safety if CR sneaks in
				if n := len(raw); n > 0 && raw[n-1] == '\r' {
					raw = raw[:n-1]
				}
				if suf := findEOLCommentSuffix(raw); len(suf) > 0 {
					// Append suffix EXACTLY as it appears in the source line,
					// including its own leading spaces/tabs before '#'.
					_, _ = w.buf.Write(suf)
					hadEOLComment = true
				}
			}
			// If we just attached an EOL comment, coalesce any immediately
			// following NL tokens (scanner/layout artifacts) into ONE newline.
			// Otherwise, DO NOT coalesce — preserve intentional blank lines.
			if hadEOLComment {
				for {
					nxt := peekItem()
					if nxt.Tok != token.NL {
						break
					}
					_ = advance() // consume extra NL
				}
			}
			// Emit the single logical newline; layout controls indent depth.
			w.nl()

		case token.Indent:
			w.indentTabs++

		case token.Dedent:
			if w.indentTabs > 0 {
				w.indentTabs--
			}

		default:
			// spacing before current token
			if shouldSpaceBefore(w, it, peekItem().Tok) {
				w.space()
			}

			// ---- emit token ----
			cat := token.TokenCategory(cur)
			switch cat {
			case token.CatIdent:
				w.tok(it.Lexeme)
				w.lineKind = _lineCode
				w.lastCodeLine = it.Line

			case token.CatLiteral:
				// Numbers usually carry lexeme; STR/FSTR/LONGSTR are empty by design.
				if it.Lexeme != "" {
					w.tok(it.Lexeme)
					w.lineKind = _lineCode
					w.lastCodeLine = it.Line
				} else {
					// Reconstruct original STRING literal exactly.
					start := byteIndex(it.Line, it.Col)
					nxt := peekItem()
					nextStart := len(src)
					if nxt.Tok != token.EOF {
						nextStart = byteIndex(nxt.Line, nxt.Col)
					}
					if start < 0 {
						start = 0
					}
					if nextStart < start || nextStart > len(src) {
						nextStart = len(src)
					}
					lit := reconstructStringLiteral(src, start, nextStart)
					w.tok(string(lit))

					// Empty-lexeme literal here is a string by design.
					if w.lineKind == _lineUnknown {
						w.lineKind = _lineStringOnly
					}
					// Do NOT update lastCodeLine when the line is string-only.
				}

			case token.CatKeyword, token.CatOperator, token.CatPunct:
				lit := cur.Lit()
				if lit == "" {
					lit = it.Lexeme
				}
				if lit != "" {
					w.tok(lit)
					w.lineKind = _lineCode
					w.lastCodeLine = it.Line
				}

			default:
				// layout handled in outer loop
			}

			// depth bookkeeping AFTER emitting
			switch cur {
			case token.LPAREN:
				w.parenDepth++
			case token.RPAREN:
				if w.parenDepth > 0 {
					w.parenDepth--
				}
			case token.LBRACK:
				w.brackDepth++
			case token.RBRACK:
				if w.brackDepth > 0 {
					w.brackDepth--
				}
			case token.LBRACE:
				w.braceDepth++
			case token.RBRACE:
				if w.braceDepth > 0 {
					w.braceDepth--
				}
			default:
				// no-op
			}

			// advance prev tokens (non-layout only)
			if token.TokenCategory(cur) != token.CatLayout {
				w.prevPrevTok = w.prevTok
				w.prevTok = cur
			}
		}
	}
}

// shouldSpaceBefore returns true if we should place a single space BEFORE `cur`.
func shouldSpaceBefore(w *writer, cur lex.Item, lookahead token.Token) bool {
	t := cur.Tok
	if w.atBOL {
		return false
	}

	// 1) Never add a space before closers or delimiters.
	switch t {
	case token.RPAREN, token.RBRACK, token.RBRACE, token.COMMA, token.COLON, token.DOT:
		return false
	default:
		// fall through
	}

	// 2) No space before call/index openers when they follow identifiers/literals/closers.
	if t == token.LPAREN && (w.prevTok == token.IDENT || token.TokenCategory(w.prevTok) == token.CatLiteral || w.prevTok == token.RPAREN || w.prevTok == token.RBRACK || w.prevTok == token.RBRACE) {
		return false
	}
	if t == token.LBRACK && (w.prevTok == token.IDENT || token.TokenCategory(w.prevTok) == token.CatLiteral || w.prevTok == token.RPAREN || w.prevTok == token.RBRACK || w.prevTok == token.RBRACE) {
		return false
	}

	// 3) No space around '.' field access (RHS case).
	if w.prevTok == token.DOT {
		return false
	}

	// 4) After a comma, add exactly one space unless next is a closer.
	if w.prevTok == token.COMMA {
		return !isCloser(t)
	}

	// 5) Space after ':' for annotations/dicts — but NOT inside slices [i:j:k] and not before NL/closer.
	if w.prevTok == token.COLON {
		if w.brackDepth > 0 {
			// likely a slice or index; keep tight "i:j"
			return false
		}
		if isCloser(t) || t == token.NL {
			return false
		}
		return true
	}

	// 6) Spaces around binary-like operators (assigns, math, compare, pipe, =>, and return-arrow "->").
	if isBinaryLike[w.prevTok] || isBinaryLike[t] || prevOrCurIsArrow(w.prevTok, t) {
		// suppress space after a unary +/− in obvious unary contexts.
		if (w.prevTok == token.MINUS || w.prevTok == token.PLUS) && isUnaryContext(w.prevPrevTok) {
			return false
		}
		return true
	}

	// 7) Keywords that prefix an expression need a space AFTER them.
	if isPrefixKeyword[w.prevTok] {
		return true
	}

	// 8) Keywords that FOLLOW a closing delimiter (e.g., in comps): "fn(x) if …"
	if token.TokenCategory(t) == token.CatKeyword {
		switch w.prevTok {
		case token.RPAREN, token.RBRACK, token.RBRACE:
			return true
		default:
			// no-op
		}
	}

	// 9) Default: space between id/kw/lit neighbors, and between '}' and next token, or before '{'.
	pc := token.TokenCategory(w.prevTok)
	cc := token.TokenCategory(t)
	if (pc == token.CatIdent || pc == token.CatKeyword || pc == token.CatLiteral || w.prevTok == token.RBRACE) &&
		(cc == token.CatIdent || cc == token.CatKeyword || cc == token.CatLiteral || t == token.LBRACE) {
		return true
	}
	return false
}

func isCloser(t token.Token) bool {
	return t == token.RPAREN || t == token.RBRACK || t == token.RBRACE
}

// Recognize when '-' or '+' is clearly unary based on the token before it.
func isUnaryContext(prevPrev token.Token) bool {
	switch prevPrev {
	case token.ASSIGN, token.DECLARE, token.LPAREN, token.LBRACK, token.LBRACE,
		token.COMMA, token.COLON,
		token.KW_return, token.KW_let, token.KW_mut, token.KW_if, token.KW_elif, token.KW_for, token.KW_while,
		token.FAT_ARROW: // => preceding an expression body
		return true
	default:
		return false
	}
}

// Treat return arrow by literal, so we don't care what the enum name is.
func prevOrCurIsArrow(prev token.Token, cur token.Token) bool {
	return prev.Lit() == "->" || cur.Lit() == "->"
}

// Table-driven sets to avoid "exhaustive switch" IDE noise.
var isBinaryLike = map[token.Token]bool{
	token.ASSIGN:     true,
	token.DECLARE:    true,
	token.PLUS:       true,
	token.MINUS:      true,
	token.STAR:       true,
	token.SLASH:      true,
	token.PERCENT:    true,
	token.PLUS_EQ:    true,
	token.MINUS_EQ:   true,
	token.STAR_EQ:    true,
	token.SLASH_EQ:   true,
	token.PERCENT_EQ: true,
	token.POW:        true,
	token.POW_EQ:     true,
	token.XOR:        true,
	token.XOR_EQ:     true,
	token.EQEQ:       true,
	token.NEQ:        true,
	token.LT:         true,
	token.LTE:        true,
	token.GT:         true,
	token.GTE:        true,
	token.IN:         true,
	token.PIPE:       true,
	token.PIPE_GT:    true, // |>
	token.FAT_ARROW:  true, // =>
	// NOTE: return-arrow "->" handled via prevOrCurIsArrow by literal
}

var isPrefixKeyword = map[token.Token]bool{
	token.KW_return: true,
	token.KW_if:     true,
	token.KW_elif:   true,
	token.KW_for:    true,
	token.KW_while:  true,
	token.KW_let:    true,
	token.KW_mut:    true,
	token.KW_not:    true,
	token.KW_await:  true,
	token.KW_as:     true,
	token.KW_from:   true,
}

// ---- EOL comment detection (local to formatter) ----

// findEOLCommentSuffix returns the trailing whitespace (if any) + "#…"
// slice from a single physical line, or nil if none should be preserved.
// Ignores '#{' and any '#' inside "..." or """...""".
func findEOLCommentSuffix(line []byte) []byte {
	if len(line) == 0 {
		return nil
	}
	// trim trailing CR if present (CRLF safety)
	if line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	i := 0
	inStr := false
	inTriple := false
	for i < len(line) {
		if line[i] == '#' && !inStr && !inTriple {
			// skip set literal opener '#{'
			if i+1 < len(line) && line[i+1] == '{' {
				i += 2
				continue
			}
			// include preceding horizontal whitespace exactly as-is
			j := i
			for j > 0 && (line[j-1] == ' ' || line[j-1] == '\t') {
				j--
			}
			suf := line[j:]
			// ensure we never return a trailing CR by accident
			if len(suf) > 0 && suf[len(suf)-1] == '\r' {
				suf = suf[:len(suf)-1]
			}
			return suf
		}
		// string open/close handling
		if !inStr && !inTriple && line[i] == '"' {
			if i+2 < len(line) && line[i+1] == '"' && line[i+2] == '"' {
				inTriple = true
				i += 3
				continue
			}
			inStr = true
			i++
			continue
		}
		if inStr {
			if line[i] == '\\' {
				i += 2
				continue
			}
			if line[i] == '"' {
				inStr = false
			}
			i++
			continue
		}
		if inTriple {
			if i+2 < len(line) && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
				inTriple = false
				i += 3
				continue
			}
			i++
			continue
		}
		i++
	}
	return nil
}

// reconstructStringLiteral returns the exact bytes of a STRING literal that
// starts at byte offset 'start'. It never crosses 'nextStart' and stops right
// after the literal's closing delimiter (", """ or their f-prefixed forms).
func reconstructStringLiteral(src []byte, start, nextStart int) []byte {
	if start >= len(src) {
		return nil
	}
	s := src[start:nextStart] // do not scan beyond next token
	// f"""..."""
	if bytes.HasPrefix(s, []byte(`f"""`)) || bytes.HasPrefix(s, []byte(`F"""`)) {
		i := 4
		if k := indexTripleQuote(s[i:]); k >= 0 {
			end := start + i + k + 3
			if end <= nextStart {
				return src[start:end]
			}
		}
		return src[start:nextStart]
	}
	// """..."""
	if bytes.HasPrefix(s, []byte(`"""`)) {
		i := 3
		if k := indexTripleQuote(s[i:]); k >= 0 {
			end := start + i + k + 3
			if end <= nextStart {
				return src[start:end]
			}
		}
		return src[start:nextStart]
	}
	// f"..." / F"..."
	if bytes.HasPrefix(s, []byte(`f"`)) || bytes.HasPrefix(s, []byte(`F"`)) {
		i := 2
		if k := indexClosingQuote(s[i:]); k >= 0 {
			end := start + i + k + 1
			if end <= nextStart {
				return src[start:end]
			}
		}
		return src[start:nextStart]
	}
	// "..."
	if bytes.HasPrefix(s, []byte(`"`)) {
		i := 1
		if k := indexClosingQuote(s[i:]); k >= 0 {
			end := start + i + k + 1
			if end <= nextStart {
				return src[start:end]
			}
		}
	}
	// Fallback: conservative clamp.
	return src[start:nextStart]
}

// find the next occurrence of """ (no escape handling needed for """ docstrings)
func indexTripleQuote(b []byte) int {
	for i := 0; i+2 < len(b); i++ {
		if b[i] == '"' && b[i+1] == '"' && b[i+2] == '"' {
			return i
		}
	}
	return -1
}

// scan to the next unescaped " (handles \" escapes)
func indexClosingQuote(b []byte) int {
	for i := 0; i < len(b); i++ {
		if b[i] == '\\' {
			i++ // skip escaped char
			continue
		}
		if b[i] == '"' {
			return i
		}
	}
	return -1
}
