package parser

import (
  "fmt"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/diag"
  "github.com/desilang/desi/compiler/internal/lexer"
)

/*** typed, span-carrying parse error ***/

type parseError struct {
  code   string
  title  string
  domain string
  key    string
  span   *ast.Span
  notes  []string
}

func (e parseError) Error() string {
  if e.code != "" {
    return e.code + ": " + e.title
  }
  return e.title
}
func (e parseError) Code() string   { return e.code }
func (e parseError) Title() string  { return e.title }
func (e parseError) Domain() string { return e.domain }
func (e parseError) Key() string    { return e.key }
func (e parseError) Span() (ast.Span, bool) {
  if e.span == nil {
    return ast.Span{}, false
  }
  return *e.span, true
}
func (e parseError) Notes() []string { return e.notes }

/*** catalog lookup ***/

func lookupIDTitle(domain, key, fallbackID, fallbackTitle string) (string, string) {
  if info, ok := diag.LookupFull(domain, key); ok {
    id := info.Entry.ID
    if id == "" {
      id = fallbackID
    }
    title := info.Entry.Title
    if title == "" {
      title = fallbackTitle
    }
    return id, title
  }
  return fallbackID, fallbackTitle
}

/*** pretty helpers ***/

func prettyKind(k lexer.TokKind) string {
  switch k {
  case lexer.TokIdent:
    return "identifier"
  case lexer.TokInt:
    return "integer literal"
  case lexer.TokFloat:
    return "float literal"
  case lexer.TokStr:
    return "string literal"
  case lexer.TokTrue, lexer.TokFalse:
    return "boolean literal"
  case lexer.TokRParen:
    return "')'"
  case lexer.TokLParen:
    return "'('"
  case lexer.TokRBrack:
    return "']'"
  case lexer.TokLBrack:
    return "'['"
  case lexer.TokRBrace:
    return "'}'"
  case lexer.TokLBrace:
    return "'{'"
  case lexer.TokComma:
    return "','"
  case lexer.TokColon:
    return "':'"
  case lexer.TokDot:
    return "'.'"
  case lexer.TokArrow:
    return "'->'"
  case lexer.TokEq:
    return "'='"
  case lexer.TokAssign:
    return "':='"
  case lexer.TokPlus:
    return "'+'"
  case lexer.TokMinus:
    return "'-'"
  case lexer.TokStar:
    return "'*'"
  case lexer.TokSlash:
    return "'/'"
  case lexer.TokPercent:
    return "'%'"
  case lexer.TokPipe:
    return "'|>'"
  case lexer.TokBang:
    return "'!'"
  case lexer.TokLt:
    return "'<'"
  case lexer.TokLe:
    return "'<='"
  case lexer.TokGt:
    return "'>'"
  case lexer.TokGe:
    return "'>='"
  case lexer.TokEqEq:
    return "'=='"
  case lexer.TokNe:
    return "'!='"
  case lexer.TokNewline:
    return "newline"
  case lexer.TokEOF:
    return "end of file"
  default:
    return k.String()
  }
}

func prettyGot(tok lexer.Token) string {
  switch tok.Kind {
  case lexer.TokIdent:
    return fmt.Sprintf("identifier %q", tok.Lex)
  case lexer.TokInt:
    return fmt.Sprintf("integer literal %q", tok.Lex)
  case lexer.TokFloat:
    return fmt.Sprintf("float literal %q", tok.Lex)
  case lexer.TokStr:
    return fmt.Sprintf("string literal %q", tok.Lex)
  case lexer.TokTrue, lexer.TokFalse:
    return fmt.Sprintf("boolean literal %q", tok.Lex)
  default:
    return prettyKind(tok.Kind)
  }
}

func spanFromTok(t lexer.Token) ast.Span {
  p := ast.Pos{Line: t.Line, Col: t.Col}
  return ast.Span{Start: p, End: p}
}

/*** constructors (DPE...) ***/

// DPE0001: unexpected token
func ErrUnexpectedToken(context string, tok lexer.Token) error {
  id, _ := lookupIDTitle("parser", "unexpected_token", "DPE0001", "unexpected token")
  title := fmt.Sprintf("unexpected token in %s: %s", context, prettyGot(tok))
  sp := spanFromTok(tok)
  return parseError{code: id, title: title, domain: "parser", key: "unexpected_token", span: &sp}
}

// DPE0002: expected a different token
func ErrExpectedToken(context string, expected lexer.TokKind, got lexer.Token) error {
  id, _ := lookupIDTitle("parser", "expected_token", "DPE0002", "expected a different token")
  title := fmt.Sprintf("expected %s, found %s", prettyKind(expected), prettyGot(got))
  sp := spanFromTok(got)
  return parseError{code: id, title: title, domain: "parser", key: "expected_token", span: &sp}
}

// DPE0003: unclosed delimiter
func ErrUnclosedDelimiter(context string, open string, want string, atTok lexer.Token) error {
  id, _ := lookupIDTitle("parser", "unclosed_delimiter", "DPE0003", "unclosed delimiter")
  title := fmt.Sprintf("unclosed %s; expected matching %s in %s", open, want, context)
  sp := spanFromTok(atTok)
  return parseError{code: id, title: title, domain: "parser", key: "unclosed_delimiter", span: &sp}
}

// DPE0004: trailing or extra token
func ErrTrailingOrExtraToken(context string, tok lexer.Token) error {
  id, _ := lookupIDTitle("parser", "trailing_or_extra_token", "DPE0004", "trailing or extra token")
  title := fmt.Sprintf("trailing or extra %s in %s", prettyGot(tok), context)
  sp := spanFromTok(tok)
  return parseError{code: id, title: title, domain: "parser", key: "trailing_or_extra_token", span: &sp}
}

// DPE0005: invalid assignment target
func ErrInvalidAssignmentTarget(context string, tok lexer.Token) error {
  id, _ := lookupIDTitle("parser", "invalid_assignment_target", "DPE0005", "invalid assignment target")
  title := fmt.Sprintf("invalid assignment target in %s: %s", context, prettyGot(tok))
  sp := spanFromTok(tok)
  return parseError{code: id, title: title, domain: "parser", key: "invalid_assignment_target", span: &sp}
}

// DPE1001: async only before 'def'
func ErrAsyncOnlyBeforeDef(got lexer.Token) error {
  id, _ := lookupIDTitle("parser", "async_before_def", "DPE1001", "async only valid before 'def'")
  title := "async is only valid immediately before 'def'"
  sp := spanFromTok(got)
  return parseError{code: id, title: title, domain: "parser", key: "async_before_def", span: &sp}
}

// DPE1002: await requires an expression
func ErrAwaitRequiresExpr(at lexer.Token) error {
  id, _ := lookupIDTitle("parser", "await_requires_expr", "DPE1002", "await requires an expression")
  title := "await requires an expression"
  sp := spanFromTok(at)
  return parseError{code: id, title: title, domain: "parser", key: "await_requires_expr", span: &sp}
}
