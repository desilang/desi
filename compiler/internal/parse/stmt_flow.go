package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// if / elif / else  (block form and one-line: "if Expr: SimpleStmt")
func (p *Parser) parseIf() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'if'
	p.next()                        // consume 'if'

	cond := p.parseExpr()
	if !p.expect(token.COLON, ":") {
		// try to continue to reduce cascading errors
		p.syncStmt()
		return &ast.ExprStmt{Expr: cond, Span: ast.JoinSpan(start, cond.SpanOf())}
	}

	// One-line vs block
	var thenBlk *ast.Block
	if p.cur.Tok == token.NL {
		p.next()
		thenBlk = p.parseBlock()
	} else {
		s := p.parseSimpleStmtInline()
		thenBlk = &ast.Block{Stmts: []ast.Stmt{s}, Span: ast.JoinSpan(s.SpanOf(), s.SpanOf())}
	}

	var elifs []ast.IfArm
	// Zero or more: elif <cond> : (block | one-line)
	for p.cur.Tok == token.KW_elif {
		p.next()
		econd := p.parseExpr()
		if !p.expect(token.COLON, ":") {
			p.syncStmt()
			break
		}
		var ebody *ast.Block
		if p.cur.Tok == token.NL {
			p.next()
			ebody = p.parseBlock()
		} else {
			es := p.parseSimpleStmtInline()
			ebody = &ast.Block{Stmts: []ast.Stmt{es}, Span: ast.JoinSpan(es.SpanOf(), es.SpanOf())}
		}
		elifs = append(elifs, ast.IfArm{Cond: econd, Body: ebody})
	}

	// Optional else
	var elseBlk *ast.Block
	if p.cur.Tok == token.KW_else {
		p.next()
		if !p.expect(token.COLON, ":") {
			p.syncStmt()
		} else if p.cur.Tok == token.NL {
			p.next()
			elseBlk = p.parseBlock()
		} else {
			es := p.parseSimpleStmtInline()
			elseBlk = &ast.Block{Stmts: []ast.Stmt{es}, Span: ast.JoinSpan(es.SpanOf(), es.SpanOf())}
		}
	}

	end := spanPos(p.file, p.cur)
	return &ast.IfStmt{
		Cond:  cond,
		Then:  thenBlk,
		Elifs: elifs,
		Else:  elseBlk,
		Span:  ast.JoinSpan(start, end),
	}
}

// while (block and one-line)
func (p *Parser) parseWhile() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'while'
	p.next()
	cond := p.parseExpr()
	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return nil
	}
	var body *ast.Block
	if p.cur.Tok == token.NL {
		p.next()
		body = p.parseBlock()
	} else {
		s := p.parseSimpleStmtInline()
		body = &ast.Block{Stmts: []ast.Stmt{s}, Span: ast.JoinSpan(s.SpanOf(), s.SpanOf())}
	}
	return &ast.WhileStmt{
		Cond: cond,
		Body: body,
		Span: ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}

// for target in iter (block and one-line)
// Target is parsed as an Expr (identifier, tuple-like, or list-like); semantics deferred.
func (p *Parser) parseFor() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'for'
	p.next()
	target := p.parseExpr()
	if !p.expect(token.KW_in, "in") {
		p.syncStmt()
		return nil
	}
	iter := p.parseExpr()
	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return nil
	}
	var body *ast.Block
	if p.cur.Tok == token.NL {
		p.next()
		body = p.parseBlock()
	} else {
		s := p.parseSimpleStmtInline()
		body = &ast.Block{Stmts: []ast.Stmt{s}, Span: ast.JoinSpan(s.SpanOf(), s.SpanOf())}
	}
	return &ast.ForStmt{
		Target: target,
		Iter:   iter,
		Body:   body,
		Span:   ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}

// parseSimpleStmtInline parses a single statement used by one-line forms.
// It consumes the trailing newline (or accepts EOF/Dedent as implicit NL).
func (p *Parser) parseSimpleStmtInline() ast.Stmt {
	switch p.cur.Tok {
	case token.KW_let:
		return p.parseLet()
	case token.KW_return:
		return p.parseReturn()
	default:
		e := p.parseExpr()

		// Same friendly error for one-line simple statements (e.g., "if x: a = 1").
		if id, ok := e.(*ast.Ident); ok && p.cur.Lexeme == "=" && id != nil {
			p.errMissingLetBeforeDecl(e.SpanOf())
			p.syncStmt()
			return &ast.ExprStmt{
				Expr: e,
				Span: ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur)),
			}
		}

		if s := p.maybeMakeAssign(e); s != nil {
			return s
		}
		span := ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))
		if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		return &ast.ExprStmt{Expr: e, Span: span}
	}
}
