package parse

import (
  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/token"
)

// maybeMakeAssign turns an already-parsed expression into an Assign/AugAssign
// statement if the next token is ':=' or an augmented op. Otherwise returns nil.
func (p *Parser) maybeMakeAssign(first ast.Expr) ast.Stmt {
  start := first.SpanOf()

  // (a, b, c) := ...    or    a, b, c := ...
  lhs := []ast.Expr{first}
  lhsEnd := start

  // Collect additional LHS items if any commas precede ':='.
  for p.cur.Tok == token.COMMA {
    p.next() // comma
    e := p.parseExpr()
    lhs = append(lhs, e)
    lhsEnd = ast.JoinSpan(lhsEnd, e.SpanOf())
  }

  switch p.cur.Tok {
  case token.DECLARE: // ':='
    p.next()
    rhs, rhsSpan := p.parseExprList()
    // Validate targets now that we know it's an assignment.
    ok := true
    for _, t := range lhs {
      if !isAssignableLHS(t) {
        p.errInvalidAssignTarget(t.SpanOf())
        ok = false
      }
    }
    // newline or implicit line end (EOF/Dedent)
    if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
      p.errExpected(spanPos(p.file, p.cur), "newline")
    }
    return &ast.AssignStmt{
      LHS:  lhs,
      RHS:  rhs,
      Span: ast.JoinSpan(start, rhsSpan),
    }

  // Aug-assign: only single left-hand expr is valid.
  case token.PLUS_EQ, token.MINUS_EQ, token.STAR_EQ, token.SLASH_EQ, token.PERCENT_EQ,
    token.POW_EQ, token.XOR_EQ:
    op := p.cur
    p.next()
    right := p.parseExpr()
    // Validate LHS
    if len(lhs) != 1 || !isAssignableLHS(lhs[0]) {
      p.errInvalidAssignTarget(lhs[0].SpanOf())
    }
    // newline or implicit line end (EOF/Dedent)
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

func (p *Parser) parseExprList() ([]ast.Expr, diagSpan ast.Span) {
  var out []ast.Expr
  if p.cur.Tok == token.NL || p.cur.Tok == token.EOF || p.cur.Tok == token.Dedent {
    // Empty RHS; we'll let checker complain later; fabricate span.
    return out, spanPos(p.file, p.cur)
  }
  first := p.parseExpr()
  out = append(out, first)
  end := first.SpanOf()

  for p.accept(token.COMMA) {
    // allow trailing comma with a graceful error; we still stop on NL/Dedent/EOF
    if p.cur.Tok == token.NL || p.cur.Tok == token.EOF || p.cur.Tok == token.Dedent {
      p.errTrailingOrExtra(spanPos(p.file, p.cur))
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
