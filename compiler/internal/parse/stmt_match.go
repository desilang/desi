package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// match Expr:
//
//	pattern: Expr
//	_:       Expr
func (p *Parser) parseMatch() *ast.MatchExpr {
	start := spanPos(p.file, p.cur) // 'match'
	p.next()                        // consume 'match'

	scr := p.parseExpr()

	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return nil
	}
	if !p.expect(token.NL, "newline") {
		p.syncStmt()
		return nil
	}
	if !p.accept(token.Indent) {
		// empty body; produce node with no arms
		return &ast.MatchExpr{Scrutinee: scr, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	}

	var arms []ast.MatchArm
	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}

		armStart := spanPos(p.file, p.cur)

		// Pattern: '_' or general expression
		var pat ast.Expr
		if p.cur.Tok == token.IDENT && p.cur.Lexeme == "_" {
			pat = &ast.Ident{Name: "_", Span: spanPos(p.file, p.cur)} // <-- pointer
			p.next()
		} else {
			pat = p.parseExpr()
		}

		if !p.expect(token.COLON, ":") {
			p.syncStmt()
			continue
		}

		res := p.parseExpr()
		end := spanPos(p.file, p.cur)

		arms = append(arms, ast.MatchArm{
			Pattern: pat,
			Result:  res,
			Span:    ast.JoinSpan(armStart, end),
		})

		// end-of-line after each arm
		if !p.accept(token.NL) {
			p.errExpected(spanPos(p.file, p.cur), "newline")
			p.syncStmt()
		}
	}

	if p.cur.Tok == token.Dedent {
		p.next()
	}

	return &ast.MatchExpr{
		Scrutinee: scr,
		Arms:      arms,
		Span:      ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}
