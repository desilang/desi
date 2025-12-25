package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parsePass parses a 'pass' statement - a no-op placeholder.
func (p *Parser) parsePass() *ast.PassStmt {
	sp := spanPos(p.file, p.cur)
	p.expect(token.KW_pass, "pass")
	return &ast.PassStmt{Span: sp}
}
