package check

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

/*
Unified, registry-backed diagnostics for the checker.

- Public functions keep the SAME names/signatures as before and still return `error`,
  but now return `diag.Diagnostic` (which implements error).
- Titles/IDs are hydrated from codes.json; fallbacks remain for resilience.
- Duplication is minimized by two generic helpers:
    TypeErrorf(...) and TypeErrorAtf(...)
  Site-specific helpers are now tiny wrappers calling those.
*/

func convSpan(sp ast.Span) diag.Span {
	return diag.Span{
		Start: diag.Pos{Line: sp.Start.Line, Col: sp.Start.Col},
		End:   diag.Pos{Line: sp.End.Line, Col: sp.End.Col},
	}
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

func mk(domain string, level diag.Level, key, id, message string, sp *ast.Span) diag.Diagnostic {
	out := diag.Diagnostic{
		Domain:  domain,
		Key:     key,
		Level:   level,
		Code:    id,
		Message: message,
	}
	if sp != nil {
		out.Span = convSpan(*sp)
	}
	return out
}

/*** Generic constructors (use these to avoid new wrappers) ***/

// TypeErrorf builds a type-domain error by key, using the registry title as the
// FIRST %s in format. Example:
//
//	TypeErrorf("type_mismatch","DTE0004","type mismatch",
//	           "%s in %s: expected %s, found %s", ctx, exp, got)
func TypeErrorf(key, fallbackID, fallbackTitle, format string, args ...any) error {
	id, title := lookupIDTitle("type", key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf(format, append([]any{title}, args...)...)
	return mk("type", diag.LevelError, key, id, msg, nil)
}

// TypeErrorAtf is the span-carrying variant.
func TypeErrorAtf(sp ast.Span, key, fallbackID, fallbackTitle, format string, args ...any) error {
	id, title := lookupIDTitle("type", key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf(format, append([]any{title}, args...)...)
	return mk("type", diag.LevelError, key, id, msg, &sp)
}

/*** warnings (codes only, used elsewhere) ***/

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

/*** Back-compat helpers (thin wrappers over the generic ones) ***/

// ErrTypeArityMismatch DTE0002: arity mismatch (grouped binding)
func ErrTypeArityMismatch(context string, names, values int) error {
	return TypeErrorf("arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
		"%s in %s: names=%d, values=%d", context, names, values)
}

// ErrUndefinedName DTE0001: undefined name
func ErrUndefinedName(name, context string) error {
	return TypeErrorf("undefined_name", "DTE0001", "undefined name",
		"%s in %s: %s", context, name)
}
func ErrUndefinedNameAt(sp ast.Span, name, context string) error {
	return TypeErrorAtf(sp, "undefined_name", "DTE0001", "undefined name",
		"%s in %s: %s", context, name)
}

// ErrRedeclaredSymbol DTE0003: redeclared symbol
func ErrRedeclaredSymbol(name, context string) error {
	return TypeErrorf("redeclared_symbol", "DTE0003", "name already defined in this scope",
		"%s in %s: %s", context, name)
}
func ErrRedeclaredSymbolAt(sp ast.Span, name, context string) error {
	return TypeErrorAtf(sp, "redeclared_symbol", "DTE0003", "name already defined in this scope",
		"%s in %s: %s", context, name)
}

// ErrTypeMismatch DTE0004: type mismatch
func ErrTypeMismatch(expected, found, context string) error {
	return TypeErrorf("type_mismatch", "DTE0004", "type mismatch",
		"%s in %s: expected %s, found %s", context, expected, found)
}

// ErrWrongReturnKind DTE0005: wrong return kind
func ErrWrongReturnKind(expected, found, context string) error {
	return TypeErrorf("wrong_return_kind", "DTE0005", "return type mismatch",
		"%s in %s: expected %s, found %s", context, expected, found)
}
func ErrWrongReturnKindAt(sp ast.Span, expected, found, context string) error {
	return TypeErrorAtf(sp, "wrong_return_kind", "DTE0005", "return type mismatch",
		"%s in %s: expected %s, found %s", context, expected, found)
}

// ErrAssignToImmutable DTE0006: assign to immutable
func ErrAssignToImmutable(name, context string) error {
	return TypeErrorf("assign_to_immutable", "DTE0006", "cannot assign to immutable variable",
		"%s in %s: %s", context, name)
}

// ErrNotPublic DTE0010: not public
func ErrNotPublic(name, context string) error {
	return TypeErrorf("not_public", "DTE0010", "symbol is not public",
		"%s in %s: %s", context, name)
}
func ErrNotPublicAt(sp ast.Span, name, context string) error {
	return TypeErrorAtf(sp, "not_public", "DTE0010", "symbol is not public",
		"%s in %s: %s", context, name)
}

// ErrPublicConstNotConst DTE0011: public const must be const
func ErrPublicConstNotConst(sp ast.Span, name string) error {
	return TypeErrorAtf(sp, "public_const_not_const", "DTE0011", "public constant must be compile-time constant",
		"%s: %s", name)
}

// ErrPubLetMutForbidden DTE0012: pub let mut forbidden
func ErrPubLetMutForbidden(sp ast.Span, name string) error {
	return TypeErrorAtf(sp, "pub_let_mut_forbidden", "DTE0012", "public let cannot be mutable",
		"%s: %s", name)
}

// ErrAwaitOutsideAsyncAt DTE1001: await outside async
func ErrAwaitOutsideAsyncAt(sp ast.Span) error {
	return TypeErrorAtf(sp, "await_outside_async", "DTE1001", "`await` is only valid inside async functions",
		"%s")
}

// ErrAwaitNonFutureAt DTE1002: await non-future
func ErrAwaitNonFutureAt(sp ast.Span, got Kind) error {
	return TypeErrorAtf(sp, "await_non_future", "DTE1002", "cannot await a non-future value",
		"%s: got %s", got.String())
}

// ErrReservedIdentifier / friends
func ErrReservedIdentifier(name, context string) error {
	return TypeErrorf("reserved_identifier", "DTE0020", "reserved keyword used as an identifier",
		"%s in %s: %s", context, name)
}
func ErrShadowBuiltin(name, context string) error {
	return TypeErrorf("shadow_builtin", "DTE0021", "cannot shadow prelude builtin",
		"%s in %s: %s", context, name)
}
func ErrImportNameConflict(name, context string) error {
	return TypeErrorf("import_name_conflict", "DTE0022", "name conflicts with imported symbol",
		"%s in %s: %s", context, name)
}
func ErrReservedIdentifierAt(sp ast.Span, name, context string) error {
	return TypeErrorAtf(sp, "reserved_identifier", "DTE0020", "reserved keyword used as an identifier",
		"%s in %s: %s", context, name)
}
func ErrShadowBuiltinAt(sp ast.Span, name, context string) error {
	return TypeErrorAtf(sp, "shadow_builtin", "DTE0021", "cannot shadow prelude builtin",
		"%s in %s: %s", context, name)
}
func ErrImportNameConflictAt(sp ast.Span, name, context string) error {
	return TypeErrorAtf(sp, "import_name_conflict", "DTE0022", "name conflicts with imported symbol",
		"%s in %s: %s", context, name)
}

// ErrUseNoneInsteadOfVoid (and helper)
func ErrUseNoneInsteadOfVoid(where string) error {
	if where == "" {
		return TypeErrorf("use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s")
	}
	return TypeErrorf("use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s in %s", where)
}
func ErrUseNoneInsteadOfVoidAt(sp ast.Span, where string) error {
	if where == "" {
		return TypeErrorAtf(sp, "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s")
	}
	return TypeErrorAtf(sp, "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'", "%s in %s", where)
}

// ErrUseNoneNotVoid (older lint)
func ErrUseNoneNotVoid(context string) error {
	if context == "" {
		return TypeErrorf("use_none_not_void", "DTE0014", "use 'none' instead of 'void'", "%s")
	}
	return TypeErrorf("use_none_not_void", "DTE0014", "use 'none' instead of 'void'", "%s in %s", context)
}

// ErrUnsupportedAssignmentTarget
func ErrUnsupportedAssignmentTarget() error {
	return TypeErrorf("unsupported_assignment_target", "DTE0031", "unsupported assignment target", "%s")
}
func ErrUnsupportedAssignmentTargetAt(sp ast.Span) error {
	return TypeErrorAtf(sp, "unsupported_assignment_target", "DTE0031", "unsupported assignment target", "%s")
}

// ErrUnknownStructType
func ErrUnknownStructType(name string) error {
	return TypeErrorf("unknown_struct_type", "DTE0032", "unknown struct type", "%s: %s", name)
}
func ErrUnknownStructTypeAt(sp ast.Span, name string) error {
	return TypeErrorAtf(sp, "unknown_struct_type", "DTE0032", "unknown struct type", "%s: %s", name)
}

// ErrFieldAccessOnNonStructAt
func ErrFieldAccessOnNonStructAt(sp ast.Span, base string) error {
	return TypeErrorAtf(sp, "field_access_on_non_struct", "DTE0033", "field access on non-struct", "%s: %q", base)
}
func ErrFieldOnNotStructAt(sp ast.Span, field, owner string) error {
	return TypeErrorAtf(sp, "field_access_on_non_struct", "DTE0033", "field access on non-struct",
		"%s: field %q on %q is not a struct", field, owner)
}

// ErrCannotAssignFieldOnNonStructAt
func ErrCannotAssignFieldOnNonStructAt(sp ast.Span, base string) error {
	return TypeErrorAtf(sp, "cannot_assign_field_on_non_struct", "DTE0034", "cannot assign to field on non-struct", "%s: %q", base)
}

// ErrIllegalDeferPositionAt
func ErrIllegalDeferPositionAt(sp ast.Span) error {
	return TypeErrorAtf(sp, "illegal_defer_position", "DTE0035", "defer is not allowed here", "%s")
}

// ErrDeferExpectsCallAt
func ErrDeferExpectsCallAt(sp ast.Span) error {
	return TypeErrorAtf(sp, "defer_expects_call", "DTE0036", "defer expects a call expression", "%s")
}

// ErrBadConditionTypeAt
func ErrBadConditionTypeAt(sp ast.Span, where string, got Kind) error {
	return TypeErrorAtf(sp, "bad_condition_type", "DTE0037", "invalid condition type",
		"%s in %s: must be bool/int, got %s", where, got.String())
}

// ErrFunctionNotValueAt / ErrTypeNotValueAt / ErrModuleAliasNotValueAt / ErrImportedFuncNotValueAt
func ErrFunctionNotValueAt(sp ast.Span, name string) error {
	return TypeErrorAtf(sp, "symbol_not_value", "DTE0038", "symbol is not a value",
		"%s: %q is a function; call it with arguments", name)
}
func ErrTypeNotValueAt(sp ast.Span, name string) error {
	return TypeErrorAtf(sp, "symbol_not_value", "DTE0038", "symbol is not a value",
		"%s: %q is a type; cannot be used as a value", name)
}
func ErrModuleAliasNotValueAt(sp ast.Span, alias, modPath string) error {
	return TypeErrorAtf(sp, "symbol_not_value", "DTE0038", "symbol is not a value",
		"%s: module alias %q (from %q) is not a value; use %s.<symbol>", alias, modPath, alias)
}
func ErrImportedFuncNotValueAt(sp ast.Span, orig, display string) error {
	return TypeErrorAtf(sp, "symbol_not_value", "DTE0038", "symbol is not a value",
		"%s: %q is a function; call it as %s(...)", orig, display)
}

// ErrUnknownSymbolInModuleAliasAt
func ErrUnknownSymbolInModuleAliasAt(sp ast.Span, sym, alias string) error {
	return TypeErrorAtf(sp, "unknown_symbol_in_module_alias", "DTE0039", "unknown symbol in module alias",
		"%s: %q in %q", sym, alias)
}

// ErrUnknownFieldOnStructAt
func ErrUnknownFieldOnStructAt(sp ast.Span, field, structName string) error {
	return TypeErrorAtf(sp, "unknown_field_on_struct", "DTE0040", "unknown field on struct",
		"%s: %q on %q", field, structName)
}

// Enum/match diagnostics
func ErrUnknownEnumVariantAt(sp ast.Span, variant, enumName string) error {
	return TypeErrorAtf(sp, "unknown_enum_variant", "DTE0041", "unknown enum variant",
		"%s: %q on enum %q", variant, enumName)
}
func ErrDuplicateMatchArmAt(sp ast.Span, variant string) error {
	return TypeErrorAtf(sp, "duplicate_match_arm", "DTE0042", "duplicate match arm",
		"%s: %q", variant)
}
func ErrPayloadlessVariantBinderAt(sp ast.Span, variant, binder string) error {
	return TypeErrorAtf(sp, "payloadless_variant_binder", "DTE0043", "payloadless variant used with a binder",
		"%s: variant %q has no payload; binder %q is invalid", variant, binder)
}
