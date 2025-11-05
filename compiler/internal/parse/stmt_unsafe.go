package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// unsafe: NL Block
func (p *Parser) parseUnsafe() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'unsafe'
	p.next()                        // consume 'unsafe'

	if !p.expect(token.COLON, "':'") {
		p.syncStmt()
		return nil
	}
	if !p.expect(token.NL, "newline") {
		p.syncStmt()
		return nil
	}

	body := p.parseBlock()
	return &ast.UnsafeBlock{
		Body: body,
		Span: ast.JoinSpan(start, body.SpanOf()),
	}
}
