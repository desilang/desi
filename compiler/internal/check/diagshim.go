package check

import (
  "fmt"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/diag"
)

/*
Unified, registry-backed diagnostics for the checker.

- Public functions keep the SAME names/signatures as before and still return `error`,
  but now return `diag.Diagnostic` (which implements error) instead of a private type.
- Titles/IDs are hydrated from codes.json; fallbacks remain for resilience.
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

func mkType(level diag.Level, key, fallbackID, fallbackTitle string, sp *ast.Span, msg string) diag.Diagnostic {
  id, title := lookupIDTitle("type", key, fallbackID, fallbackTitle)
  out := diag.Diagnostic{
    Domain:  "type",
    Key:     key,
    Level:   level,
    Code:    id,
    Message: msg,
  }
  if sp != nil {
    out.Span = convSpan(*sp)
  }
  return out
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

/*** concrete error constructors (registry-backed) ***/

// ErrTypeArityMismatch DTE0002: arity mismatch (grouped binding)
func ErrTypeArityMismatch(context string, names, values int) error {
  id, title := lookupIDTitle("type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding")
  msg := fmt.Sprintf("%s in %s: names=%d, values=%d", title, context, names, values)
  return mkType(diag.LevelError, "arity_mismatch", id, title, nil, msg)
}

// ErrUndefinedName DTE0001: undefined name
func ErrUndefinedName(name, context string) error {
  id, title := lookupIDTitle("type", "undefined_name", "DTE0001", "undefined name")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "undefined_name", id, title, nil, msg)
}
func ErrUndefinedNameAt(sp ast.Span, name, context string) error {
  id, title := lookupIDTitle("type", "undefined_name", "DTE0001", "undefined name")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "undefined_name", id, title, &sp, msg)
}

// ErrRedeclaredSymbol DTE0003: redeclared symbol
func ErrRedeclaredSymbol(name, context string) error {
  id, title := lookupIDTitle("type", "redeclared_symbol", "DTE0003", "name already defined in this scope")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "redeclared_symbol", id, title, nil, msg)
}
func ErrRedeclaredSymbolAt(sp ast.Span, name, context string) error {
  id, title := lookupIDTitle("type", "redeclared_symbol", "DTE0003", "name already defined in this scope")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "redeclared_symbol", id, title, &sp, msg)
}

// ErrTypeMismatch DTE0004: type mismatch
func ErrTypeMismatch(expected, found, context string) error {
  id, title := lookupIDTitle("type", "type_mismatch", "DTE0004", "type mismatch")
  msg := fmt.Sprintf("%s in %s: expected %s, found %s", title, context, expected, found)
  return mkType(diag.LevelError, "type_mismatch", id, title, nil, msg)
}

// ErrWrongReturnKind DTE0005: wrong return kind
func ErrWrongReturnKind(expected, found, context string) error {
  id, title := lookupIDTitle("type", "wrong_return_kind", "DTE0005", "return type mismatch")
  msg := fmt.Sprintf("%s in %s: expected %s, found %s", title, context, expected, found)
  return mkType(diag.LevelError, "wrong_return_kind", id, title, nil, msg)
}
func ErrWrongReturnKindAt(sp ast.Span, expected, found, context string) error {
  id, title := lookupIDTitle("type", "wrong_return_kind", "DTE0005", "return type mismatch")
  msg := fmt.Sprintf("%s in %s: expected %s, found %s", title, context, expected, found)
  return mkType(diag.LevelError, "wrong_return_kind", id, title, &sp, msg)
}

// ErrAssignToImmutable DTE0006: assign to immutable
func ErrAssignToImmutable(name, context string) error {
  id, title := lookupIDTitle("type", "assign_to_immutable", "DTE0006", "cannot assign to immutable variable")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "assign_to_immutable", id, title, nil, msg)
}

// ErrNotPublic DTE0010: not public
func ErrNotPublic(name, context string) error {
  id, title := lookupIDTitle("type", "not_public", "DTE0010", "symbol is not public")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "not_public", id, title, nil, msg)
}
func ErrNotPublicAt(sp ast.Span, name, context string) error {
  id, title := lookupIDTitle("type", "not_public", "DTE0010", "symbol is not public")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "not_public", id, title, &sp, msg)
}

// ErrPublicConstNotConst DTE0011: public const must be const
func ErrPublicConstNotConst(sp ast.Span, name string) error {
  id, title := lookupIDTitle("type", "public_const_not_const", "DTE0011", "public constant must be compile-time constant")
  msg := fmt.Sprintf("%s: %s", title, name)
  return mkType(diag.LevelError, "public_const_not_const", id, title, &sp, msg)
}

// ErrPubLetMutForbidden DTE0012: pub let mut forbidden
func ErrPubLetMutForbidden(sp ast.Span, name string) error {
  id, title := lookupIDTitle("type", "pub_let_mut_forbidden", "DTE0012", "public let cannot be mutable")
  msg := fmt.Sprintf("%s: %s", title, name)
  return mkType(diag.LevelError, "pub_let_mut_forbidden", id, title, &sp, msg)
}

// ErrAwaitOutsideAsyncAt DTE1001: await outside async
func ErrAwaitOutsideAsyncAt(sp ast.Span) error {
  id, title := lookupIDTitle("type", "await_outside_async", "DTE1001", "`await` is only valid inside async functions")
  msg := fmt.Sprintf("%s", title)
  return mkType(diag.LevelError, "await_outside_async", id, title, &sp, msg)
}

// ErrAwaitNonFutureAt DTE1002: await non-future
func ErrAwaitNonFutureAt(sp ast.Span, got Kind) error {
  id, title := lookupIDTitle("type", "await_non_future", "DTE1002", "cannot await a non-future value")
  msg := fmt.Sprintf("%s: got %s", title, got.String())
  return mkType(diag.LevelError, "await_non_future", id, title, &sp, msg)
}

// ErrReservedIdentifier / friends
func ErrReservedIdentifier(name, context string) error {
  id, title := lookupIDTitle("type", "reserved_identifier", "DTE0020", "reserved keyword used as an identifier")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "reserved_identifier", id, title, nil, msg)
}
func ErrShadowBuiltin(name, context string) error {
  id, title := lookupIDTitle("type", "shadow_builtin", "DTE0021", "cannot shadow prelude builtin")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "shadow_builtin", id, title, nil, msg)
}
func ErrImportNameConflict(name, context string) error {
  id, title := lookupIDTitle("type", "import_name_conflict", "DTE0022", "name conflicts with imported symbol")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "import_name_conflict", id, title, nil, msg)
}
func ErrReservedIdentifierAt(sp ast.Span, name, context string) error {
  id, title := lookupIDTitle("type", "reserved_identifier", "DTE0020", "reserved keyword used as an identifier")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "reserved_identifier", id, title, &sp, msg)
}
func ErrShadowBuiltinAt(sp ast.Span, name, context string) error {
  id, title := lookupIDTitle("type", "shadow_builtin", "DTE0021", "cannot shadow prelude builtin")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "shadow_builtin", id, title, &sp, msg)
}
func ErrImportNameConflictAt(sp ast.Span, name, context string) error {
  id, title := lookupIDTitle("type", "import_name_conflict", "DTE0022", "name conflicts with imported symbol")
  msg := fmt.Sprintf("%s in %s: %s", title, context, name)
  return mkType(diag.LevelError, "import_name_conflict", id, title, &sp, msg)
}

// ErrUseNoneInsteadOfVoid
func ErrUseNoneInsteadOfVoid(where string) error {
  id, title := lookupIDTitle("type", "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'")
  if where == "" {
    return mkType(diag.LevelError, "use_none_instead_of_void", id, title, nil, title)
  }
  msg := fmt.Sprintf("%s in %s", title, where)
  return mkType(diag.LevelError, "use_none_instead_of_void", id, title, nil, msg)
}
func ErrUseNoneInsteadOfVoidAt(sp ast.Span, where string) error {
  id, title := lookupIDTitle("type", "use_none_instead_of_void", "DTE0023", "'void' is not a type; use 'none'")
  if where == "" {
    return mkType(diag.LevelError, "use_none_instead_of_void", id, title, &sp, title)
  }
  msg := fmt.Sprintf("%s in %s", title, where)
  return mkType(diag.LevelError, "use_none_instead_of_void", id, title, &sp, msg)
}

// ErrUseNoneNotVoid (older lint)
func ErrUseNoneNotVoid(context string) error {
  id, title := lookupIDTitle("type", "use_none_not_void", "DTE0014", "use 'none' instead of 'void'")
  if context == "" {
    return mkType(diag.LevelError, "use_none_not_void", id, title, nil, title)
  }
  msg := fmt.Sprintf("%s in %s", title, context)
  return mkType(diag.LevelError, "use_none_not_void", id, title, nil, msg)
}

// ErrUnsupportedAssignmentTarget
func ErrUnsupportedAssignmentTarget() error {
  id, title := lookupIDTitle("type", "unsupported_assignment_target", "DTE0031", "unsupported assignment target")
  return mkType(diag.LevelError, "unsupported_assignment_target", id, title, nil, title)
}
func ErrUnsupportedAssignmentTargetAt(sp ast.Span) error {
  id, title := lookupIDTitle("type", "unsupported_assignment_target", "DTE0031", "unsupported assignment target")
  return mkType(diag.LevelError, "unsupported_assignment_target", id, title, &sp, title)
}

// ErrUnknownStructType
func ErrUnknownStructType(name string) error {
  id, title := lookupIDTitle("type", "unknown_struct_type", "DTE0032", "unknown struct type")
  msg := fmt.Sprintf("%s: %s", title, name)
  return mkType(diag.LevelError, "unknown_struct_type", id, title, nil, msg)
}
func ErrUnknownStructTypeAt(sp ast.Span, name string) error {
  id, title := lookupIDTitle("type", "unknown_struct_type", "DTE0032", "unknown struct type")
  msg := fmt.Sprintf("%s: %s", title, name)
  return mkType(diag.LevelError, "unknown_struct_type", id, title, &sp, msg)
}

// ErrFieldAccessOnNonStructAt
func ErrFieldAccessOnNonStructAt(sp ast.Span, base string) error {
  id, title := lookupIDTitle("type", "field_access_on_non_struct", "DTE0033", "field access on non-struct")
  msg := fmt.Sprintf("%s: %q", title, base)
  return mkType(diag.LevelError, "field_access_on_non_struct", id, title, &sp, msg)
}
func ErrFieldOnNotStructAt(sp ast.Span, field, owner string) error {
  id, title := lookupIDTitle("type", "field_access_on_non_struct", "DTE0033", "field access on non-struct")
  msg := fmt.Sprintf("%s: field %q on %q is not a struct", title, field, owner)
  return mkType(diag.LevelError, "field_access_on_non_struct", id, title, &sp, msg)
}

// ErrCannotAssignFieldOnNonStructAt
func ErrCannotAssignFieldOnNonStructAt(sp ast.Span, base string) error {
  id, title := lookupIDTitle("type", "cannot_assign_field_on_non_struct", "DTE0034", "cannot assign to field on non-struct")
  msg := fmt.Sprintf("%s: %q", title, base)
  return mkType(diag.LevelError, "cannot_assign_field_on_non_struct", id, title, &sp, msg)
}

// ErrIllegalDeferPositionAt
func ErrIllegalDeferPositionAt(sp ast.Span) error {
  id, title := lookupIDTitle("type", "illegal_defer_position", "DTE0035", "defer is not allowed here")
  return mkType(diag.LevelError, "illegal_defer_position", id, title, &sp, title)
}

// ErrDeferExpectsCallAt
func ErrDeferExpectsCallAt(sp ast.Span) error {
  id, title := lookupIDTitle("type", "defer_expects_call", "DTE0036", "defer expects a call expression")
  return mkType(diag.LevelError, "defer_expects_call", id, title, &sp, title)
}

// ErrBadConditionTypeAt
func ErrBadConditionTypeAt(sp ast.Span, where string, got Kind) error {
  id, title := lookupIDTitle("type", "bad_condition_type", "DTE0037", "invalid condition type")
  msg := fmt.Sprintf("%s in %s: must be bool/int, got %s", title, where, got.String())
  return mkType(diag.LevelError, "bad_condition_type", id, title, &sp, msg)
}

// ErrFunctionNotValueAt / ErrTypeNotValueAt / ErrModuleAliasNotValueAt / ErrImportedFuncNotValueAt
func ErrFunctionNotValueAt(sp ast.Span, name string) error {
  id, title := lookupIDTitle("type", "symbol_not_value", "DTE0038", "symbol is not a value")
  msg := fmt.Sprintf("%s: %q is a function; call it with arguments", title, name)
  return mkType(diag.LevelError, "symbol_not_value", id, title, &sp, msg)
}
func ErrTypeNotValueAt(sp ast.Span, name string) error {
  id, title := lookupIDTitle("type", "symbol_not_value", "DTE0038", "symbol is not a value")
  msg := fmt.Sprintf("%s: %q is a type; cannot be used as a value", title, name)
  return mkType(diag.LevelError, "symbol_not_value", id, title, &sp, msg)
}
func ErrModuleAliasNotValueAt(sp ast.Span, alias, modPath string) error {
  id, title := lookupIDTitle("type", "symbol_not_value", "DTE0038", "symbol is not a value")
  msg := fmt.Sprintf("%s: module alias %q (from %q) is not a value; use %s.<symbol>", title, alias, modPath, alias)
  return mkType(diag.LevelError, "symbol_not_value", id, title, &sp, msg)
}
func ErrImportedFuncNotValueAt(sp ast.Span, orig, display string) error {
  id, title := lookupIDTitle("type", "symbol_not_value", "DTE0038", "symbol is not a value")
  msg := fmt.Sprintf("%s: %q is a function; call it as %s(...)", title, orig, display)
  return mkType(diag.LevelError, "symbol_not_value", id, title, &sp, msg)
}

// ErrUnknownSymbolInModuleAliasAt
func ErrUnknownSymbolInModuleAliasAt(sp ast.Span, sym, alias string) error {
  id, title := lookupIDTitle("type", "unknown_symbol_in_module_alias", "DTE0039", "unknown symbol in module alias")
  msg := fmt.Sprintf("%s: %q in %q", title, sym, alias)
  return mkType(diag.LevelError, "unknown_symbol_in_module_alias", id, title, &sp, msg)
}

// ErrUnknownFieldOnStructAt
func ErrUnknownFieldOnStructAt(sp ast.Span, field, structName string) error {
  id, title := lookupIDTitle("type", "unknown_field_on_struct", "DTE0040", "unknown field on struct")
  msg := fmt.Sprintf("%s: %q on %q", title, field, structName)
  return mkType(diag.LevelError, "unknown_field_on_struct", id, title, &sp, msg)
}

// Enum/match diagnostics
func ErrUnknownEnumVariantAt(sp ast.Span, variant, enumName string) error {
  id, title := lookupIDTitle("type", "unknown_enum_variant", "DTE0041", "unknown enum variant")
  msg := fmt.Sprintf("%s: %q on enum %q", title, variant, enumName)
  return mkType(diag.LevelError, "unknown_enum_variant", id, title, &sp, msg)
}
func ErrDuplicateMatchArmAt(sp ast.Span, variant string) error {
  id, title := lookupIDTitle("type", "duplicate_match_arm", "DTE0042", "duplicate match arm")
  msg := fmt.Sprintf("%s: %q", title, variant)
  return mkType(diag.LevelError, "duplicate_match_arm", id, title, &sp, msg)
}
func ErrPayloadlessVariantBinderAt(sp ast.Span, variant, binder string) error {
  id, title := lookupIDTitle("type", "payloadless_variant_binder", "DTE0043", "payloadless variant used with a binder")
  msg := fmt.Sprintf("%s: variant %q has no payload; binder %q is invalid", title, variant, binder)
  return mkType(diag.LevelError, "payloadless_variant_binder", id, title, &sp, msg)
}
