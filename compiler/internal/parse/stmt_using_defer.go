package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// using <LHS> [= <Expr>] : NL Block
// LHS must be assignable (Ident | Field | Index).
func (p *Parser) parseUsing() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'using'
	p.next()                        // consume 'using'

	lhs := p.parseExpr()
	if !isAssignableLHS(lhs) {
		p.errInvalidAssignTarget(lhs.SpanOf())
	}

	var init ast.Expr
	if p.accept(token.ASSIGN) { // '=' present
		init = p.parseExpr()
	}

	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return nil
	}
	if !p.expect(token.NL, "newline") {
		p.syncStmt()
		return nil
	}

	body := p.parseBlock()
	return &ast.UsingStmt{
		Bind: lhs,
		Init: init,
		Body: body,
		Span: ast.JoinSpan(start, body.SpanOf()),
	}
}

// defer CallExpr NL
func (p *Parser) parseDefer() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'defer'
	p.next()                        // consume 'defer'

	e := p.parseExpr()
	var call *ast.CallExpr
	if ce, ok := e.(*ast.CallExpr); ok {
		call = ce
	} else {
		p.errDeferNeedsCall(e.SpanOf())
	}

	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}
	return &ast.DeferStmt{
		Call: call,
		Span: ast.JoinSpan(start, e.SpanOf()),
	}
}
