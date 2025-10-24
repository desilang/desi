package parse

import (
  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/token"
)

// parseBlock parses an indented block. It tolerates blank lines, converts an
// initial long string expression into a DocStringStmt, and joins span from
// first to last statement (or to current if empty).
func (p *Parser) parseBlock() *ast.Block {
  start := spanPos(p.file, p.cur) // we're positioned right after the NL that introduced the block

  if !p.expect(token.Indent, "indent") {
    // Produce an empty block so the callers can continue.
    return &ast.Block{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
  }

  var stmts []ast.Stmt
  for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
    // Allow blank lines inside a block.
    p.skipNLs()
    if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
      break
    }
    stmts = append(stmts, p.parseStmt())
  }
  _ = p.expect(token.Dedent, "dedent")

  // Convert a leading long string expression into a DocStringStmt.
  if len(stmts) > 0 {
    if es, ok := stmts[0].(*ast.ExprStmt); ok {
      if s, ok := es.Expr.(*ast.StrLit); ok && s.Long {
        stmts[0] = &ast.DocStringStmt{Value: s, Span: es.Span}
      }
    }
  }

  if len(stmts) == 0 {
    return &ast.Block{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
  }
  return &ast.Block{
    Stmts: stmts,
    Span:  ast.JoinSpan(stmts[0].SpanOf(), stmts[len(stmts)-1].SpanOf()),
  }
}

// parseStmt dispatches statement forms. One-line forms (if/while/for) parse
// their SimpleStmt inline without requiring Indent/Dedent.
func (p *Parser) parseStmt() ast.Stmt {
  switch p.cur.Tok {
  case token.KW_let:
    return p.parseLet()
  case token.KW_return:
    return p.parseReturn()
  case token.KW_if:
    return p.parseIf()
  case token.KW_while:
    return p.parseWhile()
  case token.KW_for:
    return p.parseFor()
  case token.KW_using:
    return p.parseUsing()
  case token.KW_defer:
    return p.parseDefer()
  case token.KW_match:
    return p.parseMatch()
  case token.KW_import: // M5
    return p.parseImport()
  case token.KW_from: // M5
    return p.parseFromImport()
  default:
    e := p.parseExpr()
    if s := p.maybeMakeAssign(e); s != nil {
      return s
    }
    // Expression statement: require newline (tolerate EOF/Dedent).
    span := ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))
    if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
      p.errExpected(spanPos(p.file, p.cur), "newline")
    }
    return &ast.ExprStmt{Expr: e, Span: span}
  }
}

// let [mut] name [: Type] = Expr
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

// return [Expr]
func (p *Parser) parseReturn() ast.Stmt {
  start := spanPos(p.file, p.cur)
  p.next() // 'return'

  // Bare return (newline immediately)
  if p.cur.Tok == token.NL {
    p.next()
    return &ast.ReturnStmt{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
  }

  // Otherwise parse a value.
  e := p.parseExpr()
  if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
    p.errExpected(spanPos(p.file, p.cur), "newline")
  }
  return &ast.ReturnStmt{
    Value: e,
    Span:  ast.JoinSpan(start, lastSpan(e, start)),
  }
}
