package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseUnsafe parses:  unsafe: NL Block
// We intentionally accept only the block form for M9C.
func (p *Parser) parseUnsafe() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'unsafe' (identifier spelling)
	p.next()                        // consume 'unsafe'

	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return &ast.UnsafeBlock{Body: &ast.Block{Span: start}, Span: start}
	}
	if !p.accept(token.NL) {
		p.errExpected(spanPos(p.file, p.cur), "newline")
		return &ast.UnsafeBlock{Body: &ast.Block{Span: start}, Span: start}
	}
	body := p.parseBlock()
	return &ast.UnsafeBlock{Body: body, Span: ast.JoinSpan(start, body.SpanOf())}
}
