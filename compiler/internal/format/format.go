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
	// 1) Parse to collect proper diagnostics. If parse fails, return diags.
	if _, diags := parse.ParseFile("<stdin>", src); len(diags) > 0 {
		return nil, diags
	}
	// 2) Token pass: scan + rewrite spacing/indent using layout tokens.
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
}

func newWriter() *writer { return &writer{atBOL: true} }

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
}

// ---- token rewriting ----

func rewrite(src []byte) []byte {
	sc := lex.NewScannerWithFile(src, "<stdin>")
	w := newWriter()

	// Build 1-based line-start table for (line,col)->byte offset mapping.
	lineStarts := make([]int, 0, 64)
	lineStarts = append(lineStarts, 0) // dummy for 0 index (unused)
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
			// Always end with exactly one newline.
			if !w.atBOL {
				w.nl()
			}
			return w.buf.Bytes()

		case token.NL:
			// collapse to single logical newline; layout (Indent/Dedent) handles depth
			// We still write a single NL (blank lines are okay, but never trailing spaces).
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

			case token.CatLiteral:
				// Numbers usually carry lexeme; STR/FSTR/LONGSTR are empty by design.
				if it.Lexeme != "" {
					w.tok(it.Lexeme)
				} else {
					// Reconstruct original literal from source slice:
					// from (it.Line,it.Col) to start of next item (or EOF).
					nxt := peekItem()
					start := byteIndex(it.Line, it.Col)
					end := len(src)
					if nxt.Tok != token.EOF {
						end = byteIndex(nxt.Line, nxt.Col)
					}
					if start < 0 {
						start = 0
					}
					if end < start || end > len(src) {
						end = len(src)
					}
					w.tok(string(src[start:end]))
				}

			case token.CatKeyword, token.CatOperator, token.CatPunct:
				lit := cur.Lit()
				if lit == "" {
					lit = it.Lexeme
				}
				if lit != "" {
					w.tok(lit)
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
