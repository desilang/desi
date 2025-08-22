package parser

import (
  "fmt"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/lexer"
)

type Parser struct {
  lx  *lexer.Lexer
  tok lexer.Token
}

func New(src string) *Parser {
  p := &Parser{lx: lexer.New(src)}
  p.next()
  return p
}

func (p *Parser) next()                   { p.tok = p.lx.Next() }
func (p *Parser) at(k lexer.TokKind) bool { return p.tok.Kind == k }
func (p *Parser) accept(k lexer.TokKind) bool {
  if p.at(k) {
    p.next()
    return true
  }
  return false
}
func (p *Parser) expect(k lexer.TokKind) (lexer.Token, error) {
  if !p.at(k) {
    return p.tok, fmt.Errorf("expected %v, got %v at %d:%d", k, p.tok.Kind, p.tok.Line, p.tok.Col)
  }
  t := p.tok
  p.next()
  return t, nil
}
func (p *Parser) skipNewlines() {
  for p.accept(lexer.TokNewline) {
  }
}

func (p *Parser) ParseFile() (*ast.File, error) {
  f := &ast.File{}
  p.skipNewlines()

  // package (optional)
  if p.accept(lexer.TokPackage) {
    name, err := p.parseDottedIdent()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokNewline); err != nil {
      return nil, err
    }
    f.Pkg = &ast.PackageDecl{Name: name}
    p.skipNewlines()
  }

  // imports
  for p.accept(lexer.TokImport) {
    path, err := p.parseDottedIdent()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokNewline); err != nil {
      return nil, err
    }
    f.Imports = append(f.Imports, ast.ImportDecl{Path: path})
    p.skipNewlines()
  }

  // decls
  for !p.at(lexer.TokEOF) {
    switch {
    case p.accept(lexer.TokDef):
      fn, err := p.parseFuncDecl()
      if err != nil {
        return nil, err
      }
      f.Decls = append(f.Decls, fn)
    default:
      for !p.at(lexer.TokNewline) && !p.at(lexer.TokEOF) {
        p.next()
      }
      p.skipNewlines()
    }
  }
  return f, nil
}

func (p *Parser) parseDottedIdent() (string, error) {
  var parts []string
  t, err := p.expect(lexer.TokIdent)
  if err != nil {
    return "", err
  }
  parts = append(parts, t.Lex)
  for p.accept(lexer.TokDot) {
    t, err := p.expect(lexer.TokIdent)
    if err != nil {
      return "", err
    }
    parts = append(parts, t.Lex)
  }
  return strings.Join(parts, "."), nil
}

func (p *Parser) parseTypeUntil(stoppers ...lexer.TokKind) (string, error) {
  stop := make(map[lexer.TokKind]bool)
  for _, k := range stoppers {
    stop[k] = true
  }
  var b strings.Builder
  depthParen, depthBrack := 0, 0
  for {
    if depthParen == 0 && depthBrack == 0 && stop[p.tok.Kind] {
      break
    }
    switch p.tok.Kind {
    case lexer.TokEOF, lexer.TokNewline, lexer.TokColon:
      return strings.TrimSpace(b.String()), nil
    case lexer.TokLParen:
      depthParen++
    case lexer.TokRParen:
      if depthParen > 0 {
        depthParen--
      }
    case lexer.TokLBrack:
      depthBrack++
    case lexer.TokRBrack:
      if depthBrack > 0 {
        depthBrack--
      }
    }
    if p.tok.Lex != "" {
      if b.Len() > 0 {
        b.WriteByte(' ')
      }
      b.WriteString(p.tok.Lex)
    } else {
      if b.Len() > 0 {
        b.WriteByte(' ')
      }
      b.WriteString(p.tok.Kind.String())
    }
    p.next()
  }
  return strings.TrimSpace(b.String()), nil
}

func (p *Parser) parseFuncDecl() (*ast.FuncDecl, error) {
  // def <name> "(" params? ")" "->" type ":" NEWLINE INDENT stmts DEDENT
  nameTok, err := p.expect(lexer.TokIdent)
  if err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokLParen); err != nil {
    return nil, err
  }

  var params []ast.Param
  if !p.accept(lexer.TokRParen) {
    for {
      id, err := p.expect(lexer.TokIdent)
      if err != nil {
        return nil, err
      }
      if _, err := p.expect(lexer.TokColon); err != nil {
        return nil, err
      }
      ty, err := p.parseTypeUntil(lexer.TokComma, lexer.TokRParen)
      if err != nil {
        return nil, err
      }
      params = append(params, ast.Param{Name: id.Lex, Type: ty})
      if p.accept(lexer.TokComma) {
        continue
      }
      _, err = p.expect(lexer.TokRParen)
      if err != nil {
        return nil, err
      }
      break
    }
  }

  if _, err := p.expect(lexer.TokArrow); err != nil {
    return nil, err
  }
  ret, err := p.parseTypeUntil(lexer.TokColon)
  if err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokColon); err != nil {
    return nil, err
  }

  body, err := p.parseBlock()
  if err != nil {
    return nil, err
  }

  return &ast.FuncDecl{
    Name:   nameTok.Lex,
    Params: params,
    Ret:    ret,
    Body:   body,
  }, nil
}

func (p *Parser) parseBlock() ([]ast.Stmt, error) {
  if _, err := p.expect(lexer.TokNewline); err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokIndent); err != nil {
    return nil, err
  }
  var body []ast.Stmt
  for !p.at(lexer.TokDedent) && !p.at(lexer.TokEOF) {
    p.skipNewlines()
    if p.at(lexer.TokDedent) || p.at(lexer.TokEOF) {
      break
    }
    s, err := p.parseStmt()
    if err != nil {
      return nil, err
    }
    body = append(body, s)
  }
  if _, err := p.expect(lexer.TokDedent); err != nil {
    return nil, err
  }
  return body, nil
}

func (p *Parser) parseStmt() (ast.Stmt, error) {
  switch {
  case p.accept(lexer.TokLet):
    mut := p.accept(lexer.TokMut)
    id, err := p.expect(lexer.TokIdent)
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokEq); err != nil {
      return nil, err
    }
    expr, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokNewline); err != nil {
      return nil, err
    }
    return &ast.LetStmt{Mutable: mut, Name: id.Lex, Expr: expr}, nil

  case p.at(lexer.TokIdent):
    save := p.tok
    p.next()
    if p.at(lexer.TokAssign) {
      p.next()
      expr, err := p.parseExpr()
      if err != nil {
        return nil, err
      }
      if _, err := p.expect(lexer.TokNewline); err != nil {
        return nil, err
      }
      return &ast.AssignStmt{Name: save.Lex, Expr: expr}, nil
    }
    lhs := &ast.IdentExpr{Name: save.Lex}
    expr, err := p.parseExprWithLHS(lhs)
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokNewline); err != nil {
      return nil, err
    }
    return &ast.ExprStmt{Expr: expr}, nil

  case p.accept(lexer.TokReturn):
    if p.at(lexer.TokNewline) {
      p.next()
      return &ast.ReturnStmt{Expr: nil}, nil
    }
    expr, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokNewline); err != nil {
      return nil, err
    }
    return &ast.ReturnStmt{Expr: expr}, nil

  case p.accept(lexer.TokIf):
    ifs, err := p.parseIfStmt()
    if err != nil {
      return nil, err
    }
    return ifs, nil

  default:
    expr, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokNewline); err != nil {
      return nil, err
    }
    return &ast.ExprStmt{Expr: expr}, nil
  }
}

func (p *Parser) parseIfStmt() (*ast.IfStmt, error) {
  cond, err := p.parseExpr()
  if err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokColon); err != nil {
    return nil, err
  }
  thenBody, err := p.parseBlock()
  if err != nil {
    return nil, err
  }
  node := &ast.IfStmt{Cond: cond, Then: thenBody}

  // zero or more elif
  for p.accept(lexer.TokElif) {
    ec, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokColon); err != nil {
      return nil, err
    }
    eb, err := p.parseBlock()
    if err != nil {
      return nil, err
    }
    node.Elifs = append(node.Elifs, ast.ElseIf{Cond: ec, Body: eb})
  }

  // optional else
  if p.accept(lexer.TokElse) {
    if _, err := p.expect(lexer.TokColon); err != nil {
      return nil, err
    }
    eb, err := p.parseBlock()
    if err != nil {
      return nil, err
    }
    node.Else = eb
  }
  return node, nil
}

/*** Expressions (Pratt parser) ***/

func (p *Parser) parseExpr() (ast.Expr, error) {
  left, err := p.parseUnary()
  if err != nil {
    return nil, err
  }
  return p.parseBinaryRHS(1, left)
}

func (p *Parser) parseExprWithLHS(lhs ast.Expr) (ast.Expr, error) {
  post, err := p.parsePostfix(lhs)
  if err != nil {
    return nil, err
  }
  return p.parseBinaryRHS(1, post)
}

func (p *Parser) parseUnary() (ast.Expr, error) {
  switch {
  case p.accept(lexer.TokMinus):
    x, err := p.parseUnary()
    if err != nil {
      return nil, err
    }
    return &ast.UnaryExpr{Op: "-", X: x}, nil
  case p.accept(lexer.TokBang):
    x, err := p.parseUnary()
    if err != nil {
      return nil, err
    }
    return &ast.UnaryExpr{Op: "!", X: x}, nil
  case p.accept(lexer.TokNot):
    x, err := p.parseUnary()
    if err != nil {
      return nil, err
    }
    return &ast.UnaryExpr{Op: "not", X: x}, nil
  default:
    return p.parsePrimary()
  }
}

func (p *Parser) parsePrimary() (ast.Expr, error) {
  if p.at(lexer.TokIdent) {
    t := p.tok
    p.next()
    return p.parsePostfix(&ast.IdentExpr{Name: t.Lex})
  }
  if p.at(lexer.TokInt) {
    t := p.tok
    p.next()
    return p.parsePostfix(&ast.IntLit{Value: t.Lex})
  }
  if p.at(lexer.TokStr) {
    t := p.tok
    p.next()
    return p.parsePostfix(&ast.StrLit{Value: t.Lex})
  }
  if p.accept(lexer.TokTrue) {
    return p.parsePostfix(&ast.BoolLit{Value: true})
  }
  if p.accept(lexer.TokFalse) {
    return p.parsePostfix(&ast.BoolLit{Value: false})
  }
  if p.accept(lexer.TokLParen) {
    e, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokRParen); err != nil {
      return nil, err
    }
    return p.parsePostfix(e)
  }
  return nil, fmt.Errorf("unexpected token in expression: %v at %d:%d", p.tok.Kind, p.tok.Line, p.tok.Col)
}

func (p *Parser) parsePostfix(base ast.Expr) (ast.Expr, error) {
  e := base
  for {
    switch {
    case p.accept(lexer.TokLParen):
      // Parse call arguments, allowing an optional trailing comma:
      // f(), f(a), f(a, b), f(a,), f(a, b,)
      var args []ast.Expr
      if !p.accept(lexer.TokRParen) {
        for {
          // allow f(a,) by accepting ')' before parsing another expr
          if p.at(lexer.TokRParen) {
            p.next()
            break
          }
          a, err := p.parseExpr()
          if err != nil {
            return nil, err
          }
          args = append(args, a)
          if p.accept(lexer.TokComma) {
            // loop to parse next arg or detect trailing ')'
            continue
          }
          if _, err := p.expect(lexer.TokRParen); err != nil {
            return nil, err
          }
          break
        }
      }
      e = &ast.CallExpr{Callee: e, Args: args}

    case p.accept(lexer.TokLBrack):
      idx, err := p.parseExpr()
      if err != nil {
        return nil, err
      }
      if _, err := p.expect(lexer.TokRBrack); err != nil {
        return nil, err
      }
      e = &ast.IndexExpr{Seq: e, Index: idx}

    case p.accept(lexer.TokDot):
      id, err := p.expect(lexer.TokIdent)
      if err != nil {
        return nil, err
      }
      e = &ast.FieldExpr{X: e, Name: id.Lex}

    default:
      return e, nil
    }
  }
}

func (p *Parser) parseBinaryRHS(minPrec int, left ast.Expr) (ast.Expr, error) {
  for {
    prec, ok := binPrec(p.tok.Kind)
    if !ok || prec < minPrec {
      return left, nil
    }
    opTok := p.tok
    p.next()

    right, err := p.parseUnary()
    if err != nil {
      return nil, err
    }

    for {
      nextPrec, ok := binPrec(p.tok.Kind)
      if !ok || nextPrec <= prec {
        break
      }
      right, err = p.parseBinaryRHS(prec+1, right)
      if err != nil {
        return nil, err
      }
    }

    left = &ast.BinaryExpr{
      Op:    opTok.Kind.String(),
      Left:  left,
      Right: right,
    }
  }
}

func binPrec(k lexer.TokKind) (int, bool) {
  switch k {
  case lexer.TokPipe:
    return 1, true // |>
  case lexer.TokOr:
    return 2, true
  case lexer.TokAnd:
    return 3, true
  case lexer.TokEqEq, lexer.TokNe:
    return 4, true
  case lexer.TokLt, lexer.TokLe, lexer.TokGt, lexer.TokGe:
    return 5, true
  case lexer.TokPlus, lexer.TokMinus:
    return 6, true
  case lexer.TokStar, lexer.TokSlash, lexer.TokPercent:
    return 7, true
  default:
    return 0, false
  }
}
