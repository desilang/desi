package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseTry parses:
//
//	try:
//	    body
//	[except [name]:]
//	    handler
//	[finally:]
//	    cleanup
//
// At least one of except or finally must be present.
func (p *Parser) parseTry() ast.Stmt {
	start := spanPos(p.file, p.cur)
	p.next() // consume 'try'

	// Expect colon + newline + block
	if !p.expect(token.COLON, ":") {
		return nil
	}
	if !p.expect(token.NL, "newline") {
		p.syncStmt()
		return nil
	}
	body := p.parseBlock()

	var exceptVar *ast.Ident
	var exceptBody *ast.Block
	var finallyBody *ast.Block

	// Skip any blank lines between blocks
	p.skipNLs()

	// Optional except clause
	if p.cur.Tok == token.KW_except {
		p.next() // consume 'except'

		// Optional error binding: except e:
		if p.cur.Tok == token.IDENT {
			exceptVar = &ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
			p.next()
		}

		if !p.expect(token.COLON, ":") {
			return nil
		}
		if !p.expect(token.NL, "newline") {
			p.syncStmt()
			return nil
		}
		exceptBody = p.parseBlock()

		// Skip blank lines before finally
		p.skipNLs()
	}

	// Optional finally clause
	if p.cur.Tok == token.KW_finally {
		p.next() // consume 'finally'

		if !p.expect(token.COLON, ":") {
			return nil
		}
		if !p.expect(token.NL, "newline") {
			p.syncStmt()
			return nil
		}
		finallyBody = p.parseBlock()
	}

	// At least one of except or finally must be present
	if exceptBody == nil && finallyBody == nil {
		p.errExpected(spanPos(p.file, p.cur), "'except' or 'finally' after try block")
		return nil
	}

	end := start
	if finallyBody != nil {
		end = finallyBody.Span
	} else if exceptBody != nil {
		end = exceptBody.Span
	}

	return &ast.TryStmt{
		Body:      body,
		ExceptVar: exceptVar,
		Except:    exceptBody,
		Finally:   finallyBody,
		Span:      ast.JoinSpan(start, end),
	}
}

// parseRaise parses: raise <expr>
func (p *Parser) parseRaise() ast.Stmt {
	start := spanPos(p.file, p.cur)
	p.next() // consume 'raise'

	val := p.parseExpr()

	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}

	return &ast.RaiseStmt{
		Value: val,
		Span:  ast.JoinSpan(start, lastSpan(val, start)),
	}
}
