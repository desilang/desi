package parse

import (
  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/token"
)

// parseBlock parses an indented block. It tolerates blank lines, converts an
// initial long string expression into a DocStringStmt, and joins span from
// first to last statement (or to current if empty).
func (p *Parser) parseBlock() *ast.Block {
  blk := &ast.Block{}

  // tolerate blank lines / comments between header NL and the actual indent
  p.skipNLs()

  if !p.expect(token.Indent, "indent") {
    return blk
  }

  // parse one or more statements until Dedent
  for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
    p.skipNLs() // allow blank lines/comments *inside* the block

    if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
      break
    }
    st := p.parseStmt()
    if st != nil {
      blk.Stmts = append(blk.Stmts, st)
    }

    // statement terminator (newline) is optional if the next token is Dedent
    if p.cur.Tok == token.NL {
      p.next()
    }
  }

  p.expect(token.Dedent, "dedent")
  return blk
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
