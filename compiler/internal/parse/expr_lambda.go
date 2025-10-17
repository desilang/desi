package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parse lambda with IDENT head:  Ident [ ":" TypeName ] "=>" Expr
// Precondition: current token is IDENT and either next is FAT_ARROW or COLON.
func (p *Parser) parseLambdaFromIdent() ast.Expr {
	// ident
	id := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	var ty *ast.TypeName
	if p.accept(token.COLON) {
		ty = p.parseTypeName()
	}

	if !p.expect(token.FAT_ARROW, "=>") {
		// Best-effort recovery: treat the ident as a plain Ident expr.
		return &id
	}
	body := p.parseExpr()

	lp := ast.LambdaParam{Name: id, Type: ty, Span: ast.JoinSpan(id.Span, lastSpan(ty))}
	return &ast.LambdaExpr{
		Params: []ast.LambdaParam{lp},
		Body:   body,
		Span:   ast.JoinSpan(id.Span, body.SpanOf()),
	}
}

// parse "(" [params] ")" "=>" Expr
// Params = Ident [ ":" TypeName ] { "," Ident [ ":" TypeName ] } [","]
func (p *Parser) parseParenLambdaOrExpr() ast.Expr {
	open := spanPos(p.file, p.cur) // '('
	p.next()

	// Try to parse lambda-parameter list shape; if it doesn't look like it,
	// fall back to the classic "( Expr )" path.
	var params []ast.LambdaParam
	tryParams := true

	if p.cur.Tok != token.RPAREN {
		for {
			if p.cur.Tok != token.IDENT {
				tryParams = False // not a lambda param list; fall back to "(expr)"
				break
			}
			name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
			p.next()

			var ty *ast.TypeName
			if p.accept(token.COLON) {
				ty = p.parseTypeName()
			}
			params = append(params, ast.LambdaParam{
				Name: name,
				Type: ty,
				Span: ast.JoinSpan(name.Span, lastSpan(ty)),
			})

			if !p.accept(token.COMMA) {
				break
			}
			if p.cur.Tok == token.RPAREN {
				// allow trailing comma
				break
			}
		}
	}

	if !p.expectClose(token.RPAREN, ")", open) {
		// broken paren group; synthesize error expr
		return &ast.Ident{Name: "<error>", Span: open}
	}

	if tryParams && p.accept(token.FAT_ARROW) {
		// It's a lambda: we already consumed ") =>"
		body := p.parseExpr()
		return &ast.LambdaExpr{
			Params: params,
			Body:   body,
			Span:   ast.JoinSpan(open, body.SpanOf()),
		}
	}

	// Not a lambda → classic parenthesized expression:
	// Re-parse as "(Expr)". Since we've already consumed the group, we can't
	// rewind; the safe approach is to treat the parsed "params" as a *single*
	// identifier expression if possible (e.g., "(x)") and otherwise
	// produce a diagnostic by parsing a dummy expression now.
	if len(params) == 1 && params[0].Type == nil {
		// "(x)" → Ident(x)
		return &params[0].Name
	}

	// Fallback: report a more helpful error and return a placeholder.
	p.errUnexpected(spanPos(p.file, p.cur), "'=>', lambda body")
	return &ast.Ident{Name: "<error>", Span: open}
}
