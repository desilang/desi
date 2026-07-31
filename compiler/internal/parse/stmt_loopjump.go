package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseBreak parses a 'break' statement.
func (p *Parser) parseBreak() *ast.BreakStmt {
	sp := spanPos(p.file, p.cur)
	p.expect(token.KW_break, "break")
	return &ast.BreakStmt{Span: sp}
}

// parseContinue parses a 'continue' statement.
func (p *Parser) parseContinue() *ast.ContinueStmt {
	sp := spanPos(p.file, p.cur)
	p.expect(token.KW_continue, "continue")
	return &ast.ContinueStmt{Span: sp}
}
