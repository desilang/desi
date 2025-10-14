package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

func (p *Parser) parseBlock() *ast.Block {
	start := spanPos(p.file, p.cur)

	// Allow blank/comment-only lines between ":" and the first indent.
	p.skipNLs()

	if !p.expect(token.Indent, "indent") {
		return &ast.Block{Span: start} // fabricate empty block; keep going
	}

	var stmts []ast.Stmt
	for {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}
		if s := p.parseStmt(); s != nil {
			stmts = append(stmts, s)
		} else {
			p.syncStmt()
		}
	}
	p.expect(token.Dedent, "dedent")
	return &ast.Block{Stmts: stmts, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.cur.Tok {
	case token.KW_async:
		// Handle bogus "async" at statement start (M1 only allows it before 'def').
		asyncSpan := spanPos(p.file, p.cur)
		// If the very next token starts a let-stmt, emit a precise diag and parse let.
		if p.peek.Tok == token.KW_let {
			p.errAsyncBeforeLet(asyncSpan)
			p.next() // consume 'async'
			return p.parseLet()
		}
		// Otherwise, say "async only valid before 'def'" and recover this line.
		p.errAsyncBeforeDef(asyncSpan)
		p.next()     // consume 'async'
		p.syncStmt() // drop rest of the line to avoid cascading errors
		return nil

	case token.KW_let:
		return p.parseLet()

	case token.KW_return:
		return p.parseReturn()

	default:
		e := p.parseExpr()
		span := lastSpan(e, spanPos(p.file, p.cur))
		// Allow EOF/Dedent to act like a newline at statement end (EOF-as-NL nicety).
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
	if !p.accept(token.NL) {
		// Also allow EOF/Dedent as statement terminators.
		if p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
	}
	return &ast.LetStmt{
		Mutable: mut, Name: name, Type: ty, Value: val,
		Span: ast.JoinSpan(start, lastSpan(val, name.Span)),
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
