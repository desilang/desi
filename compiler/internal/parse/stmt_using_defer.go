package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// using Target : NL Block
// Target can be Ident "=" CallExpr OR a general LHS (Ident|Field|Index).
// We parse an Expr and leave shape validation to later phases.
func (p *Parser) parseUsing() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'using'
	p.next()                        // consume 'using'

	target := p.parseExpr()

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
		Target: target,
		Body:   body,
		Span:   ast.JoinSpan(start, body.SpanOf()),
	}
}

// defer CallExpr NL
// Must be a call; otherwise error (DPE0002) and still build a stub so parsing continues.
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

	// Trailing NL (EOF/Dedent allowed as implicit newline)
	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}

	return &ast.DeferStmt{
		Call: call, // may be nil if not a call; checker can diagnose harder later
		Span: ast.JoinSpan(start, e.SpanOf()),
	}
}
