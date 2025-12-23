package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseSelect parses a select statement for multiplexing channel operations.
//
//	select:
//	    case binding = recv_expr:
//	        body
//	    case send_expr:
//	        body
//	    default:
//	        body
func (p *Parser) parseSelect() *ast.SelectStmt {
	start := spanPos(p.file, p.cur) // 'select'
	p.next()                        // consume 'select'

	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return nil
	}
	if !p.expect(token.NL, "newline") {
		p.syncStmt()
		return nil
	}
	if !p.accept(token.Indent) {
		// empty body
		return &ast.SelectStmt{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	}

	var cases []ast.SelectCase
	var defaultBody []ast.Stmt

	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}

		caseStart := spanPos(p.file, p.cur)

		// Use IDENT comparison for 'case' and 'default'
		if p.cur.Tok == token.IDENT && p.cur.Lexeme == "default" {
			p.next() // consume 'default'
			if !p.expect(token.COLON, ":") {
				p.syncStmt()
				continue
			}
			if !p.expect(token.NL, "newline") {
				p.syncStmt()
				continue
			}
			// Parse default body as block
			if p.accept(token.Indent) {
				defaultBody = p.parseBlockStmts()
				if p.cur.Tok == token.Dedent {
					p.next()
				}
			}
		} else if p.cur.Tok == token.IDENT && p.cur.Lexeme == "case" {
			p.next() // consume 'case'

			// Parse: [binding =] expr:
			var binding *ast.Ident
			var op ast.Expr

			// Try to parse: ident = expr
			firstExpr := p.parseExpr()
			if p.cur.Tok == token.ASSIGN {
				// It was: binding = ...
				if id, ok := firstExpr.(*ast.Ident); ok {
					binding = id
				}
				p.next() // consume '='
				op = p.parseExpr()
			} else {
				// No binding, firstExpr is the operation
				op = firstExpr
			}

			if !p.expect(token.COLON, ":") {
				p.syncStmt()
				continue
			}
			if !p.expect(token.NL, "newline") {
				p.syncStmt()
				continue
			}

			// Parse case body
			var body []ast.Stmt
			if p.accept(token.Indent) {
				body = p.parseBlockStmts()
				if p.cur.Tok == token.Dedent {
					p.next()
				}
			}

			cases = append(cases, ast.SelectCase{
				Binding: binding,
				Op:      op,
				Body:    body,
				Span:    ast.JoinSpan(caseStart, spanPos(p.file, p.cur)),
			})
		} else {
			p.errExpected(spanPos(p.file, p.cur), "'case' or 'default'")
			p.syncStmt()
		}
	}

	if p.cur.Tok == token.Dedent {
		p.next()
	}

	return &ast.SelectStmt{
		Cases:   cases,
		Default: defaultBody,
		Span:    ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}

// parseBlockStmts parses statements inside an indented block and returns them as a slice
func (p *Parser) parseBlockStmts() []ast.Stmt {
	var stmts []ast.Stmt
	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}
