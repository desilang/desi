package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/token"
)

// maybeMakeAssign turns an already-parsed expression into an Assign/AugAssign
// statement if the next token is ':=' or an augmented op. Otherwise returns nil.
func (p *Parser) maybeMakeAssign(first ast.Expr) ast.Stmt {
	start := first.SpanOf()

	// Collect LHS list if present: a, b, c := ...
	lhs := []ast.Expr{first}
	lhsEnd := start

	for p.cur.Tok == token.COMMA {
		p.next() // consume ','
		e := p.parseExpr()
		lhs = append(lhs, e)
		lhsEnd = ast.JoinSpan(lhsEnd, e.SpanOf())
	}

	switch p.cur.Tok {
	case token.ASSIGN: // '=' (reassignment)
		p.next()
		rhs, rhsSpan := p.parseExprList()
		// Validate targets - allow Ident, FieldExpr, IndexExpr
		ok := true
		for _, t := range lhs {
			if !isAssignableLHS(t) {
				p.errInvalidAssignTarget(t.SpanOf())
				ok = false
			}
		}
		// Statement terminator
		if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		_ = ok // parser continues; checker can be stricter later
		return &ast.AssignStmt{
			LHS:        lhs,
			RHS:        rhs,
			IsReassign: false, // '=' is initial binding
			Span:       ast.JoinSpan(start, rhsSpan),
		}

	case token.DECLARE: // ':=' (declaration)
		p.next()
		rhs, rhsSpan := p.parseExprList()
		// Validate targets
		ok := true
		for _, t := range lhs {
			if !isAssignableLHS(t) {
				p.errInvalidAssignTarget(t.SpanOf())
				ok = false
			}
		}
		// Statement terminator
		if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		_ = ok // parser continues; checker can be stricter later
		return &ast.AssignStmt{
			LHS:        lhs,
			RHS:        rhs,
			IsReassign: true, // ':=' is mutation
			Span:       ast.JoinSpan(start, rhsSpan),
		}

	case token.PLUS_EQ, token.MINUS_EQ, token.STAR_EQ, token.SLASH_EQ, token.PERCENT_EQ,
		token.POW_EQ, token.XOR_EQ:
		op := p.cur
		p.next()
		right := p.parseExpr()
		// Validate LHS: must be a single assignable target
		if len(lhs) != 1 || !isAssignableLHS(lhs[0]) {
			p.errInvalidAssignTarget(lhs[0].SpanOf())
		}
		if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		return &ast.AugAssignStmt{
			Op:    op.Lexeme,
			Left:  lhs[0],
			Right: right,
			Span:  ast.JoinSpan(start, lastSpan(right, spanPos(p.file, op))),
		}

	default:
		return nil
	}
}

// parseExprList parses "Expr {',' Expr}" and returns the list and its span.
func (p *Parser) parseExprList() ([]ast.Expr, diag.Span) {
	var out []ast.Expr

	// Empty RHS (we'll let checker complain later); fabricate span at cur
	if p.cur.Tok == token.NL || p.cur.Tok == token.EOF || p.cur.Tok == token.Dedent {
		return out, spanPos(p.file, p.cur)
	}

	first := p.parseExpr()
	out = append(out, first)
	end := first.SpanOf()

	for p.accept(token.COMMA) {
		// Stop on end-of-statement; we *ignore* a dangling comma here.
		if p.cur.Tok == token.NL || p.cur.Tok == token.EOF || p.cur.Tok == token.Dedent {
			break
		}
		e := p.parseExpr()
		out = append(out, e)
		end = ast.JoinSpan(end, e.SpanOf())
	}
	return out, end
}

func isAssignableLHS(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Ident, *ast.FieldExpr, *ast.IndexExpr:
		return true
	default:
		return false
	}
}
