package parser

import (
  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/lexer"
)

// parseIfStmtAt parses an if-statement starting after the 'if' token (already consumed).
// M2 supports:
//
//	if <expr> ":" ( NEWLINE INDENT stmts DEDENT | <single-line-stmt> )
//	(elif <expr> ":" (block | <single-line-stmt>))*
//	(else ":" (block | <single-line-stmt>))?
func (p *Parser) parseIfStmtAt(ifTok lexer.Token) (*ast.IfStmt, error) {
  // cond
  cond, err := p.parseExpr()
  if err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokColon); err != nil {
    return nil, err
  }

  // then body: block vs single-line
  var then []ast.Stmt
  if p.at(lexer.TokNewline) {
    then, err = p.parseBlock()
    if err != nil {
      return nil, err
    }
  } else {
    s, err := p.parseStmt()
    if err != nil {
      return nil, err
    }
    then = []ast.Stmt{s}
  }

  var elifs []ast.ElseIf
  var elseBody []ast.Stmt
  end := blockEnd(then)
  if len(then) == 1 {
    end = stmtEnd(then[0])
  }

  // zero or more elif groups
  for p.at(lexer.TokElif) {
    elifTok := p.tok
    p.next() // consume 'elif'

    econd, err := p.parseExpr()
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokColon); err != nil {
      return nil, err
    }

    var body []ast.Stmt
    if p.at(lexer.TokNewline) {
      body, err = p.parseBlock()
      if err != nil {
        return nil, err
      }
    } else {
      s, err := p.parseStmt()
      if err != nil {
        return nil, err
      }
      body = []ast.Stmt{s}
    }

    ebEnd := blockEnd(body)
    if len(body) == 1 {
      ebEnd = stmtEnd(body[0])
    }
    elifs = append(elifs, ast.ElseIf{
      Cond: econd,
      Body: body,
      Span: spanFrom(posFrom(elifTok), ebEnd),
    })
    end = ebEnd
  }

  // optional else
  if p.accept(lexer.TokElse) {
    if _, err := p.expect(lexer.TokColon); err != nil {
      return nil, err
    }

    if p.at(lexer.TokNewline) {
      elseBody, err = p.parseBlock()
      if err != nil {
        return nil, err
      }
    } else {
      s, err := p.parseStmt()
      if err != nil {
        return nil, err
      }
      elseBody = []ast.Stmt{s}
    }

    eEnd := blockEnd(elseBody)
    if len(elseBody) == 1 {
      eEnd = stmtEnd(elseBody[0])
    }
    end = eEnd
  }

  return &ast.IfStmt{
    Cond:  cond,
    Then:  then,
    Elifs: elifs,
    Else:  elseBody,
    Span:  spanFrom(posFrom(ifTok), end),
  }, nil
}

// parseWhileStmtAt parses while after 'while' consumed.
// Supports both forms:
//
//	while <expr> ":" NEWLINE INDENT stmts DEDENT
//	while <expr> ":" <single-line-stmt>
func (p *Parser) parseWhileStmtAt(whileTok lexer.Token) (*ast.WhileStmt, error) {
  cond, err := p.parseExpr()
  if err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokColon); err != nil {
    return nil, err
  }

  var body []ast.Stmt
  if p.at(lexer.TokNewline) {
    body, err = p.parseBlock()
    if err != nil {
      return nil, err
    }
  } else {
    s, err := p.parseStmt()
    if err != nil {
      return nil, err
    }
    body = []ast.Stmt{s}
  }

  end := blockEnd(body)
  if len(body) == 1 {
    end = stmtEnd(body[0])
  }
  return &ast.WhileStmt{
    Cond: cond,
    Body: body,
    Span: spanFrom(posFrom(whileTok), end),
  }, nil
}
