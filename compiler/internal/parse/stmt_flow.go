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
// Supports: for x in iter:
//
//	for x: T in iter:
//	for k, v in iter:
//	for k: K, v: V in iter:
//	for mut x: T in iter:  (mutable iteration)
//	for k: str, mut v: int in dict.items():
func (p *Parser) parseFor() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'for'
	p.next()

	// Parse typed targets: x: T or x: T, y: U or just x, y
	// Also supports: mut x: T for mutable iteration
	var targets []ast.ForTarget
	var legacyTarget ast.Expr // fallback for backward compat

	// First, try to parse as typed binding(s)
	for {
		// Check for 'mut' keyword
		isMut := false
		if p.cur.Tok == token.KW_mut {
			isMut = true
			p.next()
		}

		// Must have identifier after optional 'mut'
		if p.cur.Tok != token.IDENT {
			if isMut {
				// Error: 'mut' without identifier
				p.errExpected(spanPos(p.file, p.cur), "identifier after 'mut'")
				p.syncStmt()
				return nil
			}
			// Fallback to expression-based parsing for complex patterns
			withInAsOperator(false, func() {
				legacyTarget = p.parseExpr()
			})
			break
		}

		nameSpan := spanPos(p.file, p.cur)
		name := &ast.Ident{Name: p.cur.Lexeme, Span: nameSpan}
		p.next()

		var typeAnnotation *ast.TypeName

		// Check for type annotation `: T`
		if p.cur.Tok == token.COLON {
			p.next()
			// Peek: if next is 'in', this is not a type but the loop syntax
			// Actually we need to check if we're at `in` keyword
			if p.cur.Tok == token.KW_in {
				// No type annotation, rewind
				targets = append(targets, ast.ForTarget{Name: name, Type: nil, IsMut: isMut})
				break
			}
			// Parse type - but stop before 'in' or ','
			typeAnnotation = p.parseTypeName()
		}

		targets = append(targets, ast.ForTarget{Name: name, Type: typeAnnotation, IsMut: isMut})

		// Check for comma (more targets)
		if p.cur.Tok == token.COMMA {
			p.next()
			continue
		}
		break
	}

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
		Target:  legacyTarget,
		Targets: targets,
		Iter:    iter,
		Body:    body,
		Span:    ast.JoinSpan(start, spanPos(p.file, p.cur)),
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
