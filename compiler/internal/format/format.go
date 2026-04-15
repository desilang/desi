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

	// Track the last source line number we've processed, for detecting
	// comment-only lines in gaps between tokens.
	lastSourceLine int
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
	// Helper to emit any comment-only lines between lastSourceLine and currentLine.
	// Returns true if any comment lines were emitted.
	// We preserve the ORIGINAL indentation (counted as tab units) to ensure
	// idempotent round-tripping.
	emitSkippedCommentLines := func(currentLine int) bool {
		emitted := false
		// Process lines from lastSourceLine+1 up to (but not including) currentLine
		for ln := w.lastSourceLine + 1; ln < currentLine; ln++ {
			raw := getLineBytes(ln)
			if cmt := isCommentOnlyLine(raw); cmt != nil {
				// Count the original indentation level (tabs or spaces→tabs)
				tabCount := 0
				j := 0
				for j < len(cmt) && (cmt[j] == '\t' || cmt[j] == ' ') {
					if cmt[j] == '\t' {
						tabCount++
					}
					j++
				}
				if w.atBOL {
					for k := 0; k < tabCount; k++ {
						w.buf.WriteByte('\t')
					}
				}
				_, _ = w.buf.Write(cmt[j:])
				w.nl()
				emitted = true
			}
		}
		if currentLine-1 > w.lastSourceLine {
			w.lastSourceLine = currentLine - 1
		}
		return emitted
	}

	for {
		it = advance()
		cur = it.Tok
		switch cur {
		case token.EOF:
			// Emit any trailing comment-only lines before EOF
			emitSkippedCommentLines(it.Line)
			w.lastSourceLine = it.Line
			if !w.atBOL {
				w.nl()
			}
			return w.buf.Bytes()

		case token.NL:
			// Emit any comment-only lines skipped between the last processed
			// source line and this NL token's source line.
			hadCommentEmission := emitSkippedCommentLines(it.Line)

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
			// Skip if we just emitted comment lines (they already have newlines)
			// and there was no code content on this logical line — the NL from
			// the scanner is just marking the end of the comment-only line.
			if hadCommentEmission && !w.lineHasContent {
				// The comment emission already produced the newline.
				// Just reset lineHasContent for the next line.
			} else {
				w.nl()
			}

		case token.Indent:
			w.indentTabs++

		case token.Dedent:
			if w.indentTabs > 0 {
				w.indentTabs--
			}

		default:
			// Emit any comment-only lines that were skipped between the last token
			// and this token (scanner skips comment-only lines).
			emitSkippedCommentLines(it.Line)
			if it.Line > w.lastSourceLine {
				w.lastSourceLine = it.Line
			}

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
				isStringTok := cur == token.STR || cur == token.LONGSTR || cur == token.RAWSTR ||
					cur == token.FSTR_START || cur == token.FSTR_PART || cur == token.FSTR_END

				if !isStringTok && it.Lexeme != "" {
					// Numeric literal — lexeme carries the exact source form.
					w.tok(it.Lexeme)
					w.lineKind = _lineCode
					w.lastCodeLine = it.Line
				} else {
					// String literal — reconstruct from original source to
					// preserve quotes, escape sequences, and f-string delimiters.
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
	token.KW_return:   true,
	token.KW_if:       true,
	token.KW_elif:     true,
	token.KW_for:      true,
	token.KW_while:    true,
	token.KW_let:      true,
	token.KW_mut:      true,
	token.KW_not:      true,
	token.KW_await:    true,
	token.KW_as:       true,
	token.KW_from:     true,
	token.KW_import:   true,
	token.KW_pub:      true,
	token.KW_async:    true,
	token.KW_match:    true,
	token.KW_select:   true,
	token.KW_using:    true,
	token.KW_defer:    true,
	token.KW_unsafe:   true,
	token.KW_spawn:    true,
	token.KW_lambda:   true,
	token.KW_assert:   true,
	token.KW_const:    true,
	token.KW_static:   true,
	token.KW_def:      true,
	token.KW_class:    true,
	token.KW_struct:   true,
	token.KW_enum:     true,
	token.KW_trait:    true,
	token.KW_impl:     true,
	token.KW_type:     true,
	token.KW_try:      true,
	token.KW_except:   true,
	token.KW_finally:  true,
	token.KW_raise:    true,
}

// ---- Comment detection (local to formatter) ----

// isCommentOnlyLine returns true if the line contains only whitespace and a comment
// (starting with '#' but not '#{' for set literals). Returns the trimmed comment line
// if it's a comment-only line, or nil otherwise.
func isCommentOnlyLine(line []byte) []byte {
	if len(line) == 0 {
		return nil
	}
	// Skip leading whitespace
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return nil // blank line
	}
	// Check if first non-whitespace is '#' (but not '#{')
	if line[i] == '#' && (i+1 >= len(line) || line[i+1] != '{') {
		return line // entire line is a comment
	}
	return nil
}

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

// reconstructStringLiteral returns the exact bytes of a string literal.
// Strategy:
//  1. Detect a local opener adjacent to `start` (prefer """ / f""").
//  2. If triple: scan forward to the next """ and slice [open:end].
//  3. If single: scan forward char-by-char handling escapes to the next unescaped ".
//  4. If we can’t find an adjacent opener, search a small window backward for the
//     nearest opener (prefer triple), then do (2)/(3).
//  5. Final fallback: conservative slice to EOL to avoid generating bad syntax.
func reconstructStringLiteral(src []byte, start, nextStart int) []byte {
	// Clamp bounds.
	if start < 0 {
		start = 0
	}
	if start > len(src) {
		start = len(src)
	}
	if nextStart < 0 {
		nextStart = 0
	}
	if nextStart > len(src) {
		nextStart = len(src)
	}

	type openInfo struct {
		open   int  // index of first byte of opener (including optional f/F)
		triple bool // """ if true, otherwise single-quoted
	}

	adjacentOpen := func() (openInfo, bool) {
		// Content-start right after f"""/"""?
		if start >= 4 && (src[start-4] == 'f' || src[start-4] == 'F') &&
			start-1 < len(src) && start-3 >= 0 &&
			src[start-3] == '"' && src[start-2] == '"' && src[start-1] == '"' {
			return openInfo{open: start - 4, triple: true}, true
		}
		if start >= 3 &&
			src[start-3] == '"' && src[start-2] == '"' && src[start-1] == '"' {
			return openInfo{open: start - 3, triple: true}, true
		}
		// Content-start right after f" / " ?
		if start >= 2 && (src[start-2] == 'f' || src[start-2] == 'F') && src[start-1] == '"' {
			return openInfo{open: start - 2, triple: false}, true
		}
		if start >= 1 && src[start-1] == '"' {
			return openInfo{open: start - 1, triple: false}, true
		}
		// Token might be positioned on the opener itself.
		if start+2 < len(src) && src[start] == '"' && src[start+1] == '"' && src[start+2] == '"' {
			// f/F prefix directly before opener?
			if start-1 >= 0 && (src[start-1] == 'f' || src[start-1] == 'F') {
				return openInfo{open: start - 1, triple: true}, true
			}
			return openInfo{open: start, triple: true}, true
		}
		if start < len(src) && src[start] == '"' {
			// f/F prefix directly before opener?
			if start-1 >= 0 && (src[start-1] == 'f' || src[start-1] == 'F') {
				return openInfo{open: start - 1, triple: false}, true
			}
			return openInfo{open: start, triple: false}, true
		}
		return openInfo{}, false
	}

	searchBackwardOpen := func() (openInfo, bool) {
		const backWindow = 256
		ws := start - backWindow
		if ws < 0 {
			ws = 0
		}
		win := src[ws:start]

		// Prefer the nearest triple opener before start.
		if i := bytes.LastIndex(win, []byte(`"""`)); i >= 0 {
			open := ws + i
			// Optional f/F just before the triple.
			if open-1 >= 0 && (src[open-1] == 'f' || src[open-1] == 'F') {
				open--
			}
			return openInfo{open: open, triple: true}, true
		}
		// Otherwise the nearest single-quote opener not part of a triple.
		for i := len(win) - 1; i >= 0; i-- {
			if win[i] != '"' {
				continue
			}
			abs := ws + i
			// Skip if part of a """ opener.
			if abs-1 >= 0 && src[abs-1] == '"' && abs-2 >= 0 && src[abs-2] == '"' {
				continue
			}
			// Optional f/F prefix.
			if abs-1 >= 0 && (src[abs-1] == 'f' || src[abs-1] == 'F') {
				return openInfo{open: abs - 1, triple: false}, true
			}
			return openInfo{open: abs, triple: false}, true
		}
		return openInfo{}, false
	}

	emitTriple := func(open int) []byte {
		// Skip f + """ (4) or """ (3).
		search := open + 3
		if open < len(src) && src[open] == 'f' || (open < len(src) && src[open] == 'F') {
			search = open + 4
		}
		if search < 0 {
			search = 0
		}
		if search > len(src) {
			search = len(src)
		}
		// First closing """ after opener.
		if k := bytes.Index(src[search:], []byte(`"""`)); k >= 0 {
			end := search + k + 3
			if end > len(src) {
				end = len(src)
			}
			return src[open:end]
		}
		// Fallback: clamp to nextStart or EOF.
		end := nextStart
		if end <= open || end > len(src) {
			end = len(src)
		}
		return src[open:end]
	}

	emitSingle := func(open int) []byte {
		// Skip f" (2) or " (1).
		search := open + 1
		if open < len(src) && (src[open] == 'f' || src[open] == 'F') {
			search = open + 2
		}
		if search < 0 {
			search = 0
		}
		// Scan forward handling escapes, stop at first unescaped '"'.
		for j := search; j < len(src); j++ {
			if src[j] == '\\' {
				j++ // skip escaped char
				continue
			}
			if src[j] == '"' {
				end := j + 1
				if end > len(src) {
					end = len(src)
				}
				return src[open:end]
			}
			// Single-quoted strings should not cross physical lines.
			if src[j] == '\n' {
				break
			}
		}
		// Fallback: clamp to nextStart or EOL.
		end := nextStart
		if end <= open || end > len(src) {
			end = len(src)
		}
		if nl := bytes.IndexByte(src[open:end], '\n'); nl >= 0 {
			end = open + nl
		}
		return src[open:end]
	}

	if oi, ok := adjacentOpen(); ok {
		if oi.triple {
			return emitTriple(oi.open)
		}
		return emitSingle(oi.open)
	}
	if oi, ok := searchBackwardOpen(); ok {
		if oi.triple {
			return emitTriple(oi.open)
		}
		return emitSingle(oi.open)
	}

	// Final fallback: conservative slice to EOL so we never produce broken syntax.
	beg := start
	if beg > 0 && src[beg-1] == '"' {
		beg--
	} else if beg > 1 && (src[beg-2] == 'f' || src[beg-2] == 'F') && src[beg-1] == '"' {
		beg -= 2
	} else if beg > 3 && src[beg-3] == '"' && src[beg-2] == '"' && src[beg-1] == '"' {
		beg -= 3
		if beg > 0 && (src[beg-1] == 'f' || src[beg-1] == 'F') {
			beg--
		}
	}
	end := nextStart
	if end <= beg || end > len(src) {
		end = len(src)
	}
	if nl := bytes.IndexByte(src[beg:end], '\n'); nl >= 0 {
		end = beg + nl
	}
	return src[beg:end]
}

// lastUnescapedQuoteBefore returns the index of the last unescaped `"`
// strictly before limit, or -1 if none is found.
func lastUnescapedQuoteBefore(src []byte, limit int) int {
	for i := limit - 1; i >= 0; i-- {
		if src[i] != '"' {
			continue
		}
		// Count preceding backslashes to decide if this `"` is escaped.
		backslashes := 0
		for j := i - 1; j >= 0 && src[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return i
		}
	}
	return -1
}

// lastTripleOpenBefore finds the last occurrence of `"""` whose third quote
// index is <= atOrBefore, returning the index of the FIRST quote in that trio,
// or -1 if not found.
func lastTripleOpenBefore(src []byte, atOrBefore int) int {
	for i := atOrBefore; i-2 >= 0; i-- {
		if src[i-2] == '"' && src[i-1] == '"' && src[i] == '"' {
			return i - 2
		}
	}
	return -1
}

// prevUnescapedQuoteBefore scans backward from (idx-1) to find the previous
// unescaped `"`, returning its index or -1.
func prevUnescapedQuoteBefore(src []byte, idx int) int {
	for i := idx - 1; i >= 0; i-- {
		if src[i] != '"' {
			continue
		}
		// Unescaped?
		backslashes := 0
		for j := i - 1; j >= 0 && src[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return i
		}
	}
	return -1
}
