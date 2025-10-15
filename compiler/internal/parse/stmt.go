package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

func (p *Parser) parseBlock() *ast.Block {
	start := spanPos(p.file, p.cur)

	p.skipNLs()
	if !p.expect(token.Indent, "indent") {
		return &ast.Block{Span: start}
	}

	var stmts []ast.Stmt
	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		if p.cur.Tok == token.NL {
			p.next()
			continue
		}
		if s := p.parseStmt(); s != nil {
			stmts = append(stmts, s)
		} else {
			p.syncStmt()
		}
	}
	_ = p.expect(token.Dedent, "dedent")
	return &ast.Block{Stmts: stmts, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.cur.Tok {
	case token.KW_if:
		return p.parseIf()
	case token.KW_while:
		return p.parseWhile()
	case token.KW_for:
		return p.parseFor()
	case token.KW_using: // NEW
		return p.parseUsing() // NEW
	case token.KW_defer: // NEW
		return p.parseDefer() // NEW
	case token.KW_let:
		return p.parseLet()
	case token.KW_return:
		return p.parseReturn()
	default:
		e := p.parseExpr()
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

func (p *Parser) parseLet() ast.Stmt {
	start := spanPos(p.file, p.cur)
	p.next() // 'let'
	mut := p.accept(token.KW_mut)

	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "identifier")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	var ty *ast.TypeName
	if p.accept(token.COLON) {
		ty = p.parseTypeName()
	}

	if !p.expect(token.ASSIGN, "=") {
		return nil
	}
	val := p.parseExpr()

	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}
	return &ast.LetStmt{
		Mutable: mut,
		Name:    name,
		Type:    ty,
		Value:   val,
		Span:    ast.JoinSpan(start, lastSpan(val, start)),
	}
}

func (p *Parser) parseReturn() ast.Stmt {
	start := spanPos(p.file, p.cur)
	p.next() // 'return'
	if p.cur.Tok == token.NL {
		p.next()
		return &ast.ReturnStmt{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	}
	e := p.parseExpr()
	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}
	return &ast.ReturnStmt{Value: e, Span: ast.JoinSpan(start, lastSpan(e, start))}
}
