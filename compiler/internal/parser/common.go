package parser

import (
  "fmt"

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

func (p *Parser) expect(k lexer.TokKind) (lexer.Token, error) {
  if !p.at(k) {
    return p.tok, fmt.Errorf("expected %v, got %v at %d:%d", k, p.tok.Kind, p.tok.Line, p.tok.Col)
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
