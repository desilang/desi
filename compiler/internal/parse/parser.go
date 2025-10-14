package parse

import (
  "bytes"
  "fmt"
  "os"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/diag"
  "github.com/desilang/desi/compiler/internal/lex"
  "github.com/desilang/desi/compiler/internal/token"
)

type Parser struct {
  sc    *lex.Scanner
  file  string
  cur   lex.Item
  peek  lex.Item
  diags []diag.Diagnostic
}

func ParseFile(filename string, src []byte) (*ast.Module, []diag.Diagnostic) {
  sc := lex.NewScannerWithFile(src, filename)
  p := &Parser{sc: sc, file: filename}
  p.next() // fill cur
  p.next() // fill peek

  m := &ast.Module{File: filename, Span: spanPos(filename, p.cur)}
  // Be pragmatic for bootstrap: parse either top-level funcs OR loose statements
  // (so existing examples with expressions still print an AST).
  for p.cur.Tok != token.EOF {
    p.skipNLs()
    if p.cur.Tok == token.EOF {
      break
    }
    if p.cur.Tok == token.KW_def {
      d := p.parseFunc()
      if d != nil {
        m.Decls = append(m.Decls, d)
      }
      continue
    }
    // Allow a synthetic top-level function "__top__" that collects statements
    // when a file starts with a statement (useful for examples).
    top := &ast.FuncDecl{
      Name: ast.Ident{Name: "__top__", Span: spanPos(filename, p.cur)},
      Body: &ast.Block{Span: spanPos(filename, p.cur)},
    }
    for p.cur.Tok != token.EOF && p.cur.Tok != token.KW_def {
      p.skipNLs()
      if s := p.parseStmt(); s != nil {
        top.Body.Stmts = append(top.Body.Stmts, s)
      } else {
        p.syncStmt()
      }
      p.skipNLs()
    }
    m.Decls = append(m.Decls, top)
  }
  m.Span = ast.JoinSpan(m.Span, spanPos(filename, p.cur))
  return m, p.diags
}

/* ---------- core helpers ---------- */

func (p *Parser) next() { p.cur, p.peek = p.peek, p.sc.Next() }

func (p *Parser) at(tok token.Token) bool { return p.cur.Tok == tok }

func (p *Parser) accept(tok token.Token) bool {
  if p.at(tok) {
    p.next()
    return true
  }
  return false
}

func (p *Parser) expect(tok token.Token, label string) bool {
  if p.accept(tok) {
    return true
  }
  p.errExpected(spanPos(p.file, p.cur), label)
  return false
}

func (p *Parser) skipNLs() {
  for p.cur.Tok == token.NL {
    p.next()
  }
}

func (p *Parser) syncStmt() {
  // Consume until NL, Dedent or EOF.
  for p.cur.Tok != token.NL && p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
    p.next()
  }
  // consume one NL if present
  if p.cur.Tok == token.NL {
    p.next()
  }
}

/* ---------- declarations ---------- */

func (p *Parser) parseFunc() *ast.FuncDecl {
  start := spanPos(p.file, p.cur)
  if !p.expect(token.KW_def, "def") {
    return nil
  }
  if p.cur.Tok != token.IDENT {
    p.errExpected(spanPos(p.file, p.cur), "function name")
    return nil
  }
  name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
  p.next()

  if !p.expect(token.LPAREN, "(") {
    return nil
  }
  var params []ast.Param
  if p.cur.Tok != token.RPAREN {
    params = p.parseParams()
  }
  p.expect(token.RPAREN, ")")

  var ret *ast.TypeName
  if p.accept(token.ARROW) {
    ret = p.parseTypeName()
  }

  // Body: ":" NL Block | NL (signature only)
  var body *ast.Block
  if p.accept(token.COLON) {
    p.expect(token.NL, "newline")
    body = p.parseBlock()
  } else {
    p.expect(token.NL, "newline")
  }

  fd := &ast.FuncDecl{
    Name:    name,
    Params:  params,
    RetType: ret,
    Body:    body,
    Span:    ast.JoinSpan(start, spanPos(p.file, p.cur)),
  }
  return fd
}

func (p *Parser) parseParams() []ast.Param {
  var out []ast.Param
  for {
    if p.cur.Tok != token.IDENT {
      p.errUnexpected(spanPos(p.file, p.cur), "parameter")
      break
    }
    start := spanPos(p.file, p.cur)
    name := ast.Ident{Name: p.cur.Lexeme, Span: start}
    p.next()

    var ty *ast.TypeName
    if p.accept(token.COLON) {
      ty = p.parseTypeName()
    }

    var def ast.Expr
    if p.accept(token.ASSIGN) {
      def = p.parseExpr()
    }

    out = append(out, ast.Param{
      Name: name, Type: ty, Default: def,
      Span: ast.JoinSpan(start, lastSpan(def, name.Span)),
    })

    if !p.accept(token.COMMA) {
      break
    }
    if p.cur.Tok == token.RPAREN { // allow trailing comma
      break
    }
  }
  return out
}

func (p *Parser) parseTypeName() *ast.TypeName {
  if p.cur.Tok != token.IDENT {
    p.errExpected(spanPos(p.file, p.cur), "type name")
    return &ast.TypeName{Name: "<?>", Span: spanPos(p.file, p.cur)}
  }
  start := spanPos(p.file, p.cur)
  var b bytes.Buffer
  b.WriteString(p.cur.Lexeme)
  p.next()
  for p.accept(token.DOT) {
    if p.cur.Tok != token.IDENT {
      p.errExpected(spanPos(p.file, p.cur), "identifier after '.'")
      break
    }
    b.WriteByte('.')
    b.WriteString(p.cur.Lexeme)
    p.next()
  }
  return &ast.TypeName{Name: b.String(), Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
}

/* ---------- blocks & statements ---------- */

func (p *Parser) parseBlock() *ast.Block {
  start := spanPos(p.file, p.cur)

  // allow blank/comment-only lines at the top of a block
  p.skipNLs()

  if !p.expect(token.Indent, "indent") {
    // fabricate an empty block so we can keep going
    return &ast.Block{Span: start}
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
  case token.KW_let:
    return p.parseLet()
  case token.KW_return:
    return p.parseReturn()
  default:
    // ExprStmt
    e := p.parseExpr()
    span := lastSpan(e, spanPos(p.file, p.cur))
    if !p.accept(token.NL) {
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
    p.errExpected(spanPos(p.file, p.cur), "newline")
  }
  return &ast.LetStmt{
    Mutable: mut, Name: name, Type: ty, Value: val,
    Span: ast.JoinSpan(start, lastSpan(val, name.Span)),
  }
}

func (p *Parser) parseReturn() ast.Stmt {
  start := spanPos(p.file, p.cur)
  p.next() // 'return'
  // optional expression
  if p.cur.Tok == token.NL {
    p.next()
    return &ast.ReturnStmt{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
  }
  e := p.parseExpr()
  if !p.accept(token.NL) {
    p.errExpected(spanPos(p.file, p.cur), "newline")
  }
  return &ast.ReturnStmt{Value: e, Span: ast.JoinSpan(start, lastSpan(e, start))}
}

/* ---------- expressions (precedence & postfix) ---------- */

// order = ** > unary > * / % > + - > ^ > < <= > >= > == != > |> > and > or
func (p *Parser) parseExpr() ast.Expr { return p.parseOr() }

func (p *Parser) parseOr() ast.Expr {
  e := p.parseAnd()
  for p.cur.Tok == token.KW_or {
    op := p.cur
    p.next()
    r := p.parseAnd()
    e = &ast.BinaryExpr{Op: "or", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
  }
  return e
}

func (p *Parser) parseAnd() ast.Expr {
  e := p.parsePipe()
  for p.cur.Tok == token.KW_and {
    op := p.cur
    p.next()
    r := p.parsePipe()
    e = &ast.BinaryExpr{Op: "and", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
  }
  return e
}

func (p *Parser) parsePipe() ast.Expr {
  e := p.parseEq()
  for p.cur.Tok == token.PIPE_GT {
    op := p.cur
    p.next()
    r := p.parseEq()
    e = &ast.BinaryExpr{Op: "|>", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
  }
  return e
}

func (p *Parser) parseEq() ast.Expr {
  e := p.parseRel()
  for p.cur.Tok == token.EQEQ || p.cur.Tok == token.NEQ {
    op := p.cur
    p.next()
    r := p.parseRel()
    e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
  }
  return e
}

func (p *Parser) parseRel() ast.Expr {
  e := p.parseXor()
  for {
    switch p.cur.Tok {
    case token.LT, token.LTE, token.GT, token.GTE:
      op := p.cur
      p.next()
      r := p.parseXor()
      e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
    default:
      return e
    }
  }
}

func (p *Parser) parseXor() ast.Expr {
  e := p.parseAdd()
  for p.cur.Tok == token.XOR {
    op := p.cur
    p.next()
    r := p.parseAdd()
    e = &ast.BinaryExpr{Op: "^", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
  }
  return e
}

func (p *Parser) parseAdd() ast.Expr {
  e := p.parseMul()
  for p.cur.Tok == token.PLUS || p.cur.Tok == token.MINUS {
    op := p.cur
    p.next()
    r := p.parseMul()
    e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
  }
  return e
}

func (p *Parser) parseMul() ast.Expr {
  e := p.parsePow()
  for p.cur.Tok == token.STAR || p.cur.Tok == token.SLASH || p.cur.Tok == token.PERCENT {
    op := p.cur
    p.next()
    r := p.parsePow()
    e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
  }
  return e
}

func (p *Parser) parsePow() ast.Expr {
  // Right-assoc: a ** b ** c  == a ** (b ** c)
  left := p.parseUnary()
  if p.cur.Tok == token.POW {
    op := p.cur
    p.next()
    right := p.parsePow()
    return &ast.BinaryExpr{Op: "**", Lhs: left, Rhs: right, Span: joinTok(p.file, op, p.cur)}
  }
  return left
}

func (p *Parser) parseUnary() ast.Expr {
  switch p.cur.Tok {
  case token.MINUS:
    op := p.cur
    p.next()
    x := p.parseUnary()
    return &ast.UnaryExpr{Op: "-", X: x, Span: joinTok(p.file, op, p.cur)}
  case token.BANG:
    op := p.cur
    p.next()
    x := p.parseUnary()
    return &ast.UnaryExpr{Op: "!", X: x, Span: joinTok(p.file, op, p.cur)}
  case token.KW_not:
    op := p.cur
    p.next()
    x := p.parseUnary()
    return &ast.UnaryExpr{Op: "not", X: x, Span: joinTok(p.file, op, p.cur)}
  case token.KW_await:
    op := p.cur
    p.next()
    x := p.parseUnary()
    return &ast.UnaryExpr{Op: "await", X: x, Span: joinTok(p.file, op, p.cur)}
  default:
    return p.parsePostfix()
  }
}

func (p *Parser) parsePostfix() ast.Expr {
  e := p.parsePrimary()
  for {
    switch p.cur.Tok {
    case token.LPAREN:
      // CallSuffix: "(" [ArgList] ")"
      callStart := spanPos(p.file, p.cur)
      p.next()
      var args []ast.Expr
      if p.cur.Tok != token.RPAREN {
        for {
          args = append(args, p.parseExpr())
          if !p.accept(token.COMMA) {
            break
          }
          if p.cur.Tok == token.RPAREN {
            break
          }
        }
      }
      p.expect(token.RPAREN, ")")
      e = &ast.CallExpr{Callee: e, Args: args, Span: ast.JoinSpan(callStart, spanPos(p.file, p.cur))}
    case token.LBRACK:
      // IndexSuffix: "[" Expr "]"
      idxStart := spanPos(p.file, p.cur)
      p.next()
      idx := p.parseExpr()
      p.expect(token.RBRACK, "]")
      e = &ast.IndexExpr{X: e, Idx: idx, Span: ast.JoinSpan(idxStart, spanPos(p.file, p.cur))}
    case token.DOT:
      // FieldSuffix: "." Ident
      dotStart := spanPos(p.file, p.cur)
      p.next()
      if p.cur.Tok != token.IDENT {
        p.errExpected(spanPos(p.file, p.cur), "field name")
        return e
      }
      id := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
      p.next()
      e = &ast.FieldExpr{X: e, Name: id, Span: ast.JoinSpan(dotStart, spanPos(p.file, p.cur))}
    default:
      return e
    }
  }
}

func (p *Parser) parsePrimary() ast.Expr {
  switch p.cur.Tok {
  case token.IDENT:
    id := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
    p.next()
    return &id
  case token.INT_DEC, token.INT_HEX, token.INT_BIN, token.INT_OCT:
    it := &ast.IntLit{Text: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
    p.next()
    return it
  case token.FLOAT, token.FLOAT_EXP:
    it := &ast.FloatLit{Text: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
    p.next()
    return it
  case token.STR, token.FSTR, token.LONGSTR:
    st := &ast.StrLit{Span: spanPos(p.file, p.cur)}
    p.next()
    return st
  case token.KW_true:
    b := &ast.BoolLit{Value: true, Span: spanPos(p.file, p.cur)}
    p.next()
    return b
  case token.KW_false:
    b := &ast.BoolLit{Value: false, Span: spanPos(p.file, p.cur)}
    p.next()
    return b
  case token.KW_none:
    n := &ast.NoneLit{Span: spanPos(p.file, p.cur)}
    p.next()
    return n
  case token.LPAREN:
    p.next()
    e := p.parseExpr()
    if !p.expect(token.RPAREN, ")") {
      return e
    }
    return e
  default:
    p.errUnexpected(spanPos(p.file, p.cur), "expression")
    // fabricate an ident to continue
    id := &ast.Ident{Name: "<error>", Span: spanPos(p.file, p.cur)}
    if p.cur.Tok != token.EOF {
      p.next()
    }
    return id
  }
}

/* ---------- diagnostics ---------- */

func (p *Parser) errExpected(sp diag.Span, want string) {
  p.diags = append(p.diags, diag.Diagnostic{
    CodeID: "DPE0002", Domain: "parser",
    Title:   "expected a different token",
    Message: fmt.Sprintf("expected %s", want),
    Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
  })
}
func (p *Parser) errUnexpected(sp diag.Span, ctx string) {
  msg := "unexpected token"
  if ctx != "" {
    msg = "unexpected token while parsing " + ctx
  }
  p.diags = append(p.diags, diag.Diagnostic{
    CodeID: "DPE0001", Domain: "parser",
    Title:   "unexpected token",
    Message: msg,
    Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
  })
}

/* ---------- span helpers ---------- */

func spanPos(file string, it lex.Item) diag.Span {
  // Byte positions are 0 for bootstrap; Line/Col are accurate.
  return diag.Span{
    File:  file,
    Start: diag.Pos{Line: it.Line, Col: it.Col},
    End:   diag.Pos{Line: it.Line, Col: it.Col},
  }
}
func lastSpan(n ast.Node, fallback diag.Span) diag.Span {
  if n == nil {
    return fallback
  }
  return n.SpanOf()
}
func joinTok(file string, a, _ lex.Item) diag.Span {
  return diag.Span{
    File:  file,
    Start: diag.Pos{Line: a.Line, Col: a.Col},
    End:   diag.Pos{Line: a.Line, Col: a.Col},
  }
}

// Dev helper to read a whole file (not used in parser entry point).
func mustRead(path string) []byte {
  b, err := os.ReadFile(path)
  if err != nil {
    panic(err)
  }
  return b
}
