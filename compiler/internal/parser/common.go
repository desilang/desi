package parser

import (
	"github.com/desilang/desi/compiler/internal/lexer"
)

// TokenSource is any source that can produce lexer.Token values.
// It lets us plug in either the built-in Go lexer or an adapter
// that replays tokens from the Desi lexer via the bridge.
type TokenSource interface {
	Next() lexer.Token
}

type Parser struct {
	lx  TokenSource
	tok lexer.Token
}

// New keeps the legacy code path: built-in Go lexer over source text.
func New(src string) *Parser {
	return NewFromSource(lexer.New(src))
}

// NewFromSource allows callers to supply any TokenSource (e.g., the
// lexbridge NDJSON adapter) instead of the built-in Go lexer.
func NewFromSource(src TokenSource) *Parser {
	p := &Parser{lx: src}
	p.next()
	return p
}

func (p *Parser) next()                   { p.tok = p.lx.Next() }
func (p *Parser) at(k lexer.TokKind) bool { return p.tok.Kind == k }

func (p *Parser) accept(k lexer.TokKind) bool {
	if p.at(k) {
		p.next()
		return true
	}
	return false
}

// expect now emits catalog-backed diagnostics.
// It uses DPE0002 by default, and upgrades to DPE0003 when the pattern
// looks like an unclosed delimiter (e.g., expected ) ] } but got EOF or newline).
func (p *Parser) expect(k lexer.TokKind) (lexer.Token, error) {
	if !p.at(k) {
		// Choose message
		got := p.tok

		// Try to detect unclosed delimiter cases.
		// We do not track a full delimiter stack here; we just map the
		// expected closer to its opener for a clearer message.
		open := ""
		want := ""
		switch k {
		case lexer.TokRParen:
			open, want = "(", ")"
		case lexer.TokRBrack:
			open, want = "[", "]"
		case lexer.TokRBrace:
			open, want = "{", "}"
		}

		// If we were expecting a closing delimiter and hit EOF or a newline,
		// surface it as "unclosed delimiter".
		if open != "" && (got.Kind == lexer.TokEOF || got.Kind == lexer.TokNewline) {
			return got, ErrUnclosedDelimiter("expect", open, want, got)
		}

		// Otherwise, generic expected-vs-got
		return got, ErrExpectedToken("expect", k, got)
	}
	t := p.tok
	p.next()
	return t, nil
}

func (p *Parser) skipNewlines() {
	for p.accept(lexer.TokNewline) {
	}
}

// Operator precedence
func binPrec(k lexer.TokKind) (int, bool) {
	switch k {
	case lexer.TokPipe:
		return 1, true // |>
	case lexer.TokOr:
		return 2, true
	case lexer.TokAnd:
		return 3, true
	case lexer.TokEqEq, lexer.TokNe:
		return 4, true
	case lexer.TokLt, lexer.TokLe, lexer.TokGt, lexer.TokGe:
		return 5, true
	case lexer.TokPlus, lexer.TokMinus:
		return 6, true
	case lexer.TokStar, lexer.TokSlash, lexer.TokPercent:
		return 7, true
	default:
		return 0, false
	}
}
