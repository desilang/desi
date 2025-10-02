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
func CodeUnusedVariable() string        { return warnCode("warn", "unused_variable", "DW0001") }
func CodeShadowedVariable() string      { return warnCode("warn", "shadowed_variable", "DW0002") }
func CodeUnreachableCode() string       { return warnCode("warn", "unreachable_code", "DW0004") }
func CodeMissingExplicitReturn() string { return warnCode("warn", "missing_explicit_return", "DW0006") }

// ErrAssignToImmutable produces DTE0006 for attempts to assign to a let-bound name.
func ErrAssignToImmutable(name, context string) error {
	id, title := lookupIDTitle("type", "assign_to_immutable", "DTE0006", "cannot assign to immutable variable")
	msg := fmt.Sprintf("%s in %s: %s", title, context, name)
	return typedError{code: id, title: msg, domain: "type", key: "assign_to_immutable"}
}

// -----------------------------------------------------------------------------
// NEW (M10/B): visibility + public-const diagnostics
// -----------------------------------------------------------------------------

// ErrNotPublic => DTE0010 (span-less)
func ErrNotPublic(name, context string) error {
	id, title := lookupIDTitle("type", "not_public", "DTE0010", "symbol is not public")
	msg := fmt.Sprintf("%s in %s: %s", title, context, name)
	return typedError{code: id, title: msg, domain: "type", key: "not_public"}
}

// ErrPublicConstNotConst => DTE0011
func ErrPublicConstNotConst(sp ast.Span, name string) error {
	id, title := lookupIDTitle("type", "public_const_not_const", "DTE0011", "public constant must be compile-time constant")
	msg := fmt.Sprintf("%s: %s", title, name)
	return typedError{
		code:   id,
		title:  msg,
		domain: "type",
		key:    "public_const_not_const",
		span:   &sp,
	}
}

// ErrPubLetMutForbidden => DTE0012
func ErrPubLetMutForbidden(sp ast.Span, name string) error {
	id, title := lookupIDTitle("type", "pub_let_mut_forbidden", "DTE0012", "public let cannot be mutable")
	msg := fmt.Sprintf("%s: %s", title, name)
	return typedError{
		code:   id,
		title:  msg,
		domain: "type",
		key:    "pub_let_mut_forbidden",
		span:   &sp,
	}
}

// ErrNotPublicAt => DTE0010 (with primary span)
func ErrNotPublicAt(sp ast.Span, name, context string) error {
	id, title := lookupIDTitle("type", "not_public", "DTE0010", "symbol is not public")
	msg := fmt.Sprintf("%s in %s: %s", title, context, name)
	return typedError{
		code:   id,
		title:  msg,
		domain: "type",
		key:    "not_public",
		span:   &sp,
	}
}

// -----------------------------------------------------------------------------
// NEW (M11): async/await diagnostics
// -----------------------------------------------------------------------------

// DTE1001: await_outside_async
func ErrAwaitOutsideAsyncAt(sp ast.Span) error {
	id, title := lookupIDTitle("type", "await_outside_async", "DTE1001", "`await` is only valid inside async functions")
	return typedError{
		code:   id,
		title:  title,
		domain: "type",
		key:    "await_outside_async",
		span:   &sp,
	}
}

// DTE1002: await_non_future (with got-kind)
func ErrAwaitNonFutureAt(sp ast.Span, got Kind) error {
	id, title := lookupIDTitle("type", "await_non_future", "DTE1002", "cannot await a non-future value")
	msg := fmt.Sprintf("%s: got %s", title, got.String())
	return typedError{
		code:   id,
		title:  msg,
		domain: "type",
		key:    "await_non_future",
		span:   &sp,
	}
}

// -----------------------------------------------------------------------------
// NEW: identifier hygiene helpers (cataloged)
// -----------------------------------------------------------------------------

// ErrReservedIdentifier => DTE0020
func ErrReservedIdentifier(name, context string) error {
	id, title := lookupIDTitle("type", "reserved_identifier", "DTE0020", "reserved keyword used as an identifier")
	msg := fmt.Sprintf("%s in %s: %s", title, context, name)
	return typedError{code: id, title: msg, domain: "type", key: "reserved_identifier"}
}

// ErrShadowBuiltin => DTE0021
func ErrShadowBuiltin(name, context string) error {
	id, title := lookupIDTitle("type", "shadow_builtin", "DTE0021", "cannot shadow prelude builtin")
	msg := fmt.Sprintf("%s in %s: %s", title, context, name)
	return typedError{code: id, title: msg, domain: "type", key: "shadow_builtin"}
}

// ErrImportNameConflict => DTE0022
func ErrImportNameConflict(name, context string) error {
	id, title := lookupIDTitle("type", "import_name_conflict", "DTE0022", "name conflicts with imported symbol")
	msg := fmt.Sprintf("%s in %s: %s", title, context, name)
	return typedError{code: id, title: msg, domain: "type", key: "import_name_conflict"}
}

// ErrUseNoneInsteadOfVoid (span-less)
func ErrUseNoneInsteadOfVoid(where string) error {
	id, title := lookupIDTitle("type", "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'")
	msg := title
	if where != "" {
		msg = fmt.Sprintf("%s in %s", title, where)
	}
	return typedError{code: id, title: msg, domain: "type", key: "use_none_instead_of_void"}
}

// ErrUseNoneInsteadOfVoidAt (span-carrying)
func ErrUseNoneInsteadOfVoidAt(sp ast.Span, where string) error {
	id, title := lookupIDTitle("type", "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'")
	msg := title
	if where != "" {
		msg = fmt.Sprintf("%s in %s", title, where)
	}
	return typedError{
		code:   id,
		title:  msg,
		domain: "type",
		key:    "use_none_instead_of_void",
		span:   &sp,
	}
}
