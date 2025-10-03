// compiler/internal/parser/pdiag.go
package parser

import (
  "fmt"

  "github.com/desilang/desi/compiler/internal/diag"
  "github.com/desilang/desi/compiler/internal/lexer"
)

// parseError carries a catalog code and a rendered message.
type parseError struct {
  code   string
  title  string
  domain string
  key    string
}

func (e parseError) Error() string {
  if e.code != "" {
    return e.code + ": " + e.title
  }
  return e.title
}

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

/* ---------- pretty printers ---------- */

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
    // keywords show as their surface text (def/if/while/…)
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

// ErrUnexpectedToken => DPE0001
func ErrUnexpectedToken(context string, tok lexer.Token) error {
  id, title := lookupIDTitle("parser", "unexpected_token", "DPE0001", "unexpected token")
  msg := fmt.Sprintf("%s in %s at %d:%d: found %s", title, context, tok.Line, tok.Col, prettyGot(tok))
  return parseError{code: id, title: msg, domain: "parser", key: "unexpected_token"}
}

// ErrExpectedToken => DPE0002
func ErrExpectedToken(context string, expected lexer.TokKind, got lexer.Token) error {
  id, title := lookupIDTitle("parser", "expected_token", "DPE0002", "expected a different token")
  msg := fmt.Sprintf("%s in %s at %d:%d: expected %s, found %s",
    title, context, got.Line, got.Col, prettyKind(expected), prettyGot(got))
  return parseError{code: id, title: msg, domain: "parser", key: "expected_token"}
}

// ErrUnclosedDelimiter => DPE0003
func ErrUnclosedDelimiter(context string, open string, want string, atTok lexer.Token) error {
  id, title := lookupIDTitle("parser", "unclosed_delimiter", "DPE0003", "unclosed delimiter")
  msg := fmt.Sprintf("%s in %s near %d:%d: opened with %s but did not find %s",
    title, context, atTok.Line, atTok.Col, open, want)
  return parseError{code: id, title: msg, domain: "parser", key: "unclosed_delimiter"}
}

// ErrTrailingOrExtraToken => DPE0004
func ErrTrailingOrExtraToken(context string, tok lexer.Token) error {
  id, title := lookupIDTitle("parser", "trailing_or_extra_token", "DPE0004", "trailing or extra token")
  msg := fmt.Sprintf("%s in %s at %d:%d: %s", title, context, tok.Line, tok.Col, prettyGot(tok))
  return parseError{code: id, title: msg, domain: "parser", key: "trailing_or_extra_token"}
}

// ErrInvalidAssignmentTarget => DPE0005
func ErrInvalidAssignmentTarget(context string, tok lexer.Token) error {
  id, title := lookupIDTitle("parser", "invalid_assignment_target", "DPE0005", "invalid assignment target")
  msg := fmt.Sprintf("%s in %s at %d:%d: %s", title, context, tok.Line, tok.Col, prettyGot(tok))
  return parseError{code: id, title: msg, domain: "parser", key: "invalid_assignment_target"}
}

// NEW (M11) — ErrAsyncOnlyBeforeDef => DPE1001
func ErrAsyncOnlyBeforeDef(got lexer.Token) error {
  id, title := lookupIDTitle("parser", "async_before_def", "DPE1001", "async only valid before 'def'")
  msg := fmt.Sprintf("%s at %d:%d: %s", title, got.Line, got.Col, prettyGot(got))
  return parseError{code: id, title: msg, domain: "parser", key: "async_before_def"}
}

// NEW (M11) — ErrAwaitRequiresExpr => DPE1002
func ErrAwaitRequiresExpr(at lexer.Token) error {
  id, title := lookupIDTitle("parser", "await_requires_expr", "DPE1002", "await requires an expression")
  msg := fmt.Sprintf("%s at %d:%d", title, at.Line, at.Col)
  return parseError{code: id, title: msg, domain: "parser", key: "await_requires_expr"}
}
