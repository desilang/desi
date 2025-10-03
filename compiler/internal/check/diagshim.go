package check

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

/*** core error type ***/

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

/*** generic constructors (DRY helpers) ***/

// typeErrf formats a type-domain error message. The first %s in format is
// always the catalog title; remaining args follow.
func typeErrf(key, fallbackID, fallbackTitle, format string, args ...any) error {
	id, title := lookupIDTitle("type", key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf(format, append([]any{title}, args...)...)
	return typedError{code: id, title: msg, domain: "type", key: key}
}

// typeErrAtf is the span-carrying variant.
func typeErrAtf(sp ast.Span, key, fallbackID, fallbackTitle, format string, args ...any) error {
	id, title := lookupIDTitle("type", key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf(format, append([]any{title}, args...)...)
	return typedError{code: id, title: msg, domain: "type", key: key, span: &sp}
}

// common pattern: "title in <context>: <name>"
func nameInContext(key, id, title, name, context string) error {
	return typeErrf(key, id, title, "%s in %s: %s", context, name)
}
func nameInContextAt(sp ast.Span, key, id, title, name, context string) error {
	return typeErrAtf(sp, key, id, title, "%s in %s: %s", context, name)
}

/*** warnings ***/

func warnCode(domain, key, fallbackID string) string {
	if info, ok := diag.LookupFull(domain, key); ok && info.Entry.ID != "" {
		return info.Entry.ID
	}
	return fallbackID
}

func CodeUnusedVariable() string        { return warnCode("warn", "unused_variable", "DW0001") }
func CodeShadowedVariable() string      { return warnCode("warn", "shadowed_variable", "DW0002") }
func CodeUnreachableCode() string       { return warnCode("warn", "unreachable_code", "DW0004") }
func CodeMissingExplicitReturn() string { return warnCode("warn", "missing_explicit_return", "DW0006") }

/*** concrete error constructors (now thin wrappers) ***/

// ErrTypeArityMismatch DTE0002: arity mismatch (grouped binding)
func ErrTypeArityMismatch(context string, names, values int) error {
	return typeErrf("arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
		"%s in %s: names=%d, values=%d", context, names, values)
}

// ErrUndefinedName DTE0001: undefined name
func ErrUndefinedName(name, context string) error {
	return nameInContext("undefined_name", "DTE0001", "undefined name", name, context)
}
func ErrUndefinedNameAt(sp ast.Span, name, context string) error {
	return nameInContextAt(sp, "undefined_name", "DTE0001", "undefined name", name, context)
}

// ErrRedeclaredSymbol DTE0003: redeclared symbol
func ErrRedeclaredSymbol(name, context string) error {
	return nameInContext("redeclared_symbol", "DTE0003", "name already defined in this scope", name, context)
}

// ErrTypeMismatch DTE0004: type mismatch
func ErrTypeMismatch(expected, found, context string) error {
	return typeErrf("type_mismatch", "DTE0004", "type mismatch",
		"%s in %s: expected %s, found %s", context, expected, found)
}

// ErrWrongReturnKind DTE0005: wrong return kind
func ErrWrongReturnKind(expected, found, context string) error {
	return typeErrf("wrong_return_kind", "DTE0005", "return type mismatch",
		"%s in %s: expected %s, found %s", context, expected, found)
}
func ErrWrongReturnKindAt(sp ast.Span, expected, found, context string) error {
	return typeErrAtf(sp, "wrong_return_kind", "DTE0005", "return type mismatch",
		"%s in %s: expected %s, found %s", context, expected, found)
}

// ErrAssignToImmutable DTE0006: assign to immutable
func ErrAssignToImmutable(name, context string) error {
	return nameInContext("assign_to_immutable", "DTE0006", "cannot assign to immutable variable", name, context)
}

// ErrNotPublic DTE0010: not public
func ErrNotPublic(name, context string) error {
	return nameInContext("not_public", "DTE0010", "symbol is not public", name, context)
}
func ErrNotPublicAt(sp ast.Span, name, context string) error {
	return nameInContextAt(sp, "not_public", "DTE0010", "symbol is not public", name, context)
}

// ErrPublicConstNotConst DTE0011: public const must be const
func ErrPublicConstNotConst(sp ast.Span, name string) error {
	return typeErrAtf(sp, "public_const_not_const", "DTE0011", "public constant must be compile-time constant",
		"%s: %s", name)
}

// ErrPubLetMutForbidden DTE0012: pub let mut forbidden
func ErrPubLetMutForbidden(sp ast.Span, name string) error {
	return typeErrAtf(sp, "pub_let_mut_forbidden", "DTE0012", "public let cannot be mutable",
		"%s: %s", name)
}

// ErrAwaitOutsideAsyncAt DTE1001: await outside async
func ErrAwaitOutsideAsyncAt(sp ast.Span) error {
	return typeErrAtf(sp, "await_outside_async", "DTE1001", "`await` is only valid inside async functions",
		"%s")
}

// ErrAwaitNonFutureAt DTE1002: await non-future
func ErrAwaitNonFutureAt(sp ast.Span, got Kind) error {
	return typeErrAtf(sp, "await_non_future", "DTE1002", "cannot await a non-future value",
		"%s: got %s", got.String())
}

// ErrReservedIdentifier DTE0020/21/22: identifier hygiene
func ErrReservedIdentifier(name, context string) error {
	return nameInContext("reserved_identifier", "DTE0020", "reserved keyword used as an identifier", name, context)
}
func ErrShadowBuiltin(name, context string) error {
	return nameInContext("shadow_builtin", "DTE0021", "cannot shadow prelude builtin", name, context)
}
func ErrImportNameConflict(name, context string) error {
	return nameInContext("import_name_conflict", "DTE0022", "name conflicts with imported symbol", name, context)
}
func ErrReservedIdentifierAt(sp ast.Span, name, context string) error {
	return nameInContextAt(sp, "reserved_identifier", "DTE0020", "reserved keyword used as an identifier", name, context)
}
func ErrShadowBuiltinAt(sp ast.Span, name, context string) error {
	return nameInContextAt(sp, "shadow_builtin", "DTE0021", "cannot shadow prelude builtin", name, context)
}
func ErrImportNameConflictAt(sp ast.Span, name, context string) error {
	return nameInContextAt(sp, "import_name_conflict", "DTE0022", "name conflicts with imported symbol", name, context)
}

// ErrUseNoneInsteadOfVoid DTE0023: 'void' is not a type; use 'none'
func ErrUseNoneInsteadOfVoid(where string) error {
	// Allow optional context text after the title.
	if where == "" {
		return typeErrf("use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s")
	}
	return typeErrf("use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s in %s", where)
}
func ErrUseNoneInsteadOfVoidAt(sp ast.Span, where string) error {
	if where == "" {
		return typeErrAtf(sp, "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s")
	}
	return typeErrAtf(sp, "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s in %s", where)
}

// ErrUseNoneNotVoid DTE0014: use 'none' not 'void' (older lint)
func ErrUseNoneNotVoid(context string) error {
	if context == "" {
		return typeErrf("use_none_not_void", "DTE0014", "use 'none' instead of 'void'", "%s")
	}
	return typeErrf("use_none_not_void", "DTE0014", "use 'none' instead of 'void'", "%s in %s", context)
}
