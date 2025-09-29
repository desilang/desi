package check

import (
  "fmt"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/diag"
)

// typedError carries a diagnostic code, a rendered title/message,
// and (domain,key) so the CLI can fetch help text from the catalog.
// It can optionally carry a primary span and notes for pretty rendering.
type typedError struct {
  code   string
  title  string
  domain string
  key    string

  span  *ast.Span
  notes []string
}

func (e typedError) Error() string {
  if e.code != "" {
    return e.code + ": " + e.title
  }
  return e.title
}
func (e typedError) Code() string   { return e.code }
func (e typedError) Title() string  { return e.title }
func (e typedError) Domain() string { return e.domain }
func (e typedError) Key() string    { return e.key }

// Span returns the primary span when present; used by the CLI pretty renderer.
func (e typedError) Span() (ast.Span, bool) {
  if e.span == nil {
    return ast.Span{}, false
  }
  return *e.span, true
}

// Notes returns any extra note lines attached to this diagnostic.
func (e typedError) Notes() []string { return e.notes }

// lookupIDTitle consults the diag catalog; falls back if missing.
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

// typedErr constructs a typedError using catalog (or fallbacks) and contextualizes it.
// This variant is tailored for arity messages and keeps your existing call sites stable.
func typedErr(domain, key, fallbackID, fallbackTitle, context string, names, values int) error {
  id, title := lookupIDTitle(domain, key, fallbackID, fallbackTitle)
  msg := fmt.Sprintf("%s in %s: names=%d, values=%d", title, context, names, values)
  return typedError{code: id, title: msg, domain: domain, key: key}
}

// warnCode returns the catalog ID for a warning or a fallback.
func warnCode(domain, key, fallbackID string) string {
  if info, ok := diag.LookupFull(domain, key); ok && info.Entry.ID != "" {
    return info.Entry.ID
  }
  return fallbackID
}

// -----------------------------------------------------------------------------
// Span-aware helpers for common TYPE errors (keep old ones intact)
// -----------------------------------------------------------------------------

// ErrTypeArityMismatch produces DTE0002 (arity mismatch) with contextual counts.
func ErrTypeArityMismatch(context string, names, values int) error {
  return typedErr("type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding", context, names, values)
}

// ErrUndefinedName produces DTE0001 with the missing identifier spelled out.
func ErrUndefinedName(name, context string) error {
  id, title := lookupIDTitle("type", "undefined_name", "DTE0001", "undefined name")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return typedError{code: id, title: msg, domain: "type", key: "undefined_name"}
}

// ErrUndefinedNameAt is the span-carrying variant of ErrUndefinedName.
func ErrUndefinedNameAt(sp ast.Span, name, context string) error {
  id, title := lookupIDTitle("type", "undefined_name", "DTE0001", "undefined name")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return typedError{
    code:   id,
    title:  msg,
    domain: "type",
    key:    "undefined_name",
    span:   &sp,
  }
}

// ErrRedeclaredSymbol produces DTE0003 when a name is redefined in the same scope.
func ErrRedeclaredSymbol(name, context string) error {
  id, title := lookupIDTitle("type", "redeclared_symbol", "DTE0003", "name already defined in this scope")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return typedError{code: id, title: msg, domain: "type", key: "redeclared_symbol"}
}

// ErrTypeMismatch produces DTE0004 including expected vs found descriptions.
func ErrTypeMismatch(expected, found, context string) error {
  id, title := lookupIDTitle("type", "type_mismatch", "DTE0004", "type mismatch")
  msg := fmt.Sprintf("%s in %s: expected %s, found %s", title, context, expected, found)
  return typedError{code: id, title: msg, domain: "type", key: "type_mismatch"}
}

// ErrWrongReturnKind produces DTE0005 when a function's return expression mismatches the declared type.
func ErrWrongReturnKind(expected, found, context string) error {
  id, title := lookupIDTitle("type", "wrong_return_kind", "DTE0005", "return type mismatch")
  msg := fmt.Sprintf("%s in %s: expected %s, found %s", title, context, expected, found)
  return typedError{code: id, title: msg, domain: "type", key: "wrong_return_kind"}
}

// Warning code getters (IDs only). Use these to tag Warning models consistently.

func CodeUnusedVariable() string {
  return warnCode("warn", "unused_variable", "DW0001")
}

func CodeShadowedVariable() string {
  return warnCode("warn", "shadowed_variable", "DW0002")
}

func CodeUnreachableCode() string {
  return warnCode("warn", "unreachable_code", "DW0004")
}

func CodeMissingExplicitReturn() string {
  return warnCode("warn", "missing_explicit_return", "DW0006")
}

// ErrAssignToImmutable produces DTE0006 for attempts to assign to a let-bound name.
func ErrAssignToImmutable(name, context string) error {
  id, title := lookupIDTitle("type", "assign_to_immutable", "DTE0006", "cannot assign to immutable variable")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return typedError{code: id, title: msg, domain: "type", key: "assign_to_immutable"}
}

// -----------------------------------------------------------------------------
// NEW (M10-B): visibility + pub-constant helpers (diagnostics only)
// -----------------------------------------------------------------------------

// ErrNotPublic -> DTE0010
func ErrNotPublic(name, context string) error {
  id, title := lookupIDTitle("type", "not_public", "DTE0010", "symbol is not public")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return typedError{code: id, title: msg, domain: "type", key: "not_public"}
}

// ErrPublicConstNotConst -> DTE0011
func ErrPublicConstNotConst(name string) error {
  id, title := lookupIDTitle("type", "public_const_not_const", "DTE0011", "public constant is not a compile-time constant")
  msg := fmt.Sprintf("%s: %s", title, name)
  return typedError{code: id, title: msg, domain: "type", key: "public_const_not_const"}
}

// ErrPubLetMutForbidden -> DTE0012
func ErrPubLetMutForbidden(name string) error {
  id, title := lookupIDTitle("type", "pub_let_mut_forbidden", "DTE0012", "public mutable variable is forbidden")
  msg := fmt.Sprintf("%s: %s", title, name)
  return typedError{code: id, title: msg, domain: "type", key: "pub_let_mut_forbidden"}
}
