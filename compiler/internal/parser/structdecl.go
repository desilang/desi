package parser

import (
  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/lexer"
)

// parseStructDeclAt expects we've just consumed 'struct'.
//
// Grammar (Stage-0):
//
//	struct <Ident> ":" NEWLINE INDENT
//	  <Ident> ":" <type> NEWLINE
//	  ...
//	DEDENT
func (p *Parser) parseStructDeclAt(structTok lexer.Token) (*ast.StructDecl, error) {
  nameTok, err := p.expect(lexer.TokIdent)
  if err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokColon); err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokNewline); err != nil {
    return nil, err
  }
  if _, err := p.expect(lexer.TokIndent); err != nil {
    return nil, err
  }

  var fields []ast.Field
  for !p.at(lexer.TokDedent) && !p.at(lexer.TokEOF) {
    p.skipNewlines()
    if p.at(lexer.TokDedent) || p.at(lexer.TokEOF) {
      break
    }

    // field: ident ":" type NEWLINE
    fnameTok, err := p.expect(lexer.TokIdent)
    if err != nil {
      return nil, err
    }
    if _, err := p.expect(lexer.TokColon); err != nil {
      return nil, err
    }
    ty, err := p.parseTypeUntil(lexer.TokNewline)
    if err != nil {
      return nil, err
    }
    nl, err := p.expect(lexer.TokNewline)
    if err != nil {
      return nil, err
    }
    fields = append(fields, ast.Field{
      Name: fnameTok.Lex,
      Type: ty,
      Span: spanTok(fnameTok, nl),
    })
  }

  ded, err := p.expect(lexer.TokDedent)
  if err != nil {
    return nil, err
  }

  return &ast.StructDecl{
    Name:   nameTok.Lex,
    Fields: fields,
    Span:   spanTok(structTok, ded),
    // Pub is set by caller (file-level) when 'pub' modifier is present.
  }, nil
}
