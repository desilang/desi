package parser

import (
  "fmt"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) parseStmt() (ast.Stmt, error) {
  // Surface lexer errors immediately inside statements
  if p.at(lexer.TokErr) {
    t := p.tok
    return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
  }

  switch {
  case p.at(lexer.TokLet):
    letTok := p.tok
    p.next() // consume 'let'
    return p.parseLetStmtAt(letTok)

  case p.at(lexer.TokIdent):
    return p.parseAssignOrExpr()

  case p.at(lexer.TokReturn):
    retTok := p.tok
    p.next()
    if p.at(lexer.TokNewline) {
      nl := p.tok
      p.next()
      return &ast.ReturnStmt{Expr: nil, Span: spanTok(retTok, nl)}, nil
    }
    expr, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    nl, err := p.expect(lexer.TokNewline)
    if err != nil {
      return nil, err
    }
    return &ast.ReturnStmt{Expr: expr, Span: spanTok(retTok, nl)}, nil

  case p.at(lexer.TokIf):
    ifTok := p.tok
    p.next()
    ifs, err := p.parseIfStmtAt(ifTok)
    if err != nil {
      return nil, err
    }
    return ifs, nil

  case p.at(lexer.TokWhile):
    whileTok := p.tok
    p.next()
    ws, err := p.parseWhileStmtAt(whileTok)
    if err != nil {
      return nil, err
    }
    return ws, nil

  case p.at(lexer.TokDefer):
    dTok := p.tok
    p.next()
    expr, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    nl, err := p.expect(lexer.TokNewline)
    if err != nil {
      return nil, err
    }
    return &ast.DeferStmt{Call: expr, Span: spanTok(dTok, nl)}, nil

  case p.at(lexer.TokElif), p.at(lexer.TokElse):
    return nil, ErrUnexpectedToken("statement", p.tok)

  default:
    expr, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    nl, err := p.expect(lexer.TokNewline)
    if err != nil {
      return nil, err
    }
    return &ast.ExprStmt{Expr: expr, Span: spanFrom(exprStart(expr), endPosFrom(nl))}, nil
  }
}
