package diag_test

import (
  "strings"
  "testing"

  "github.com/desilang/desi/compiler/internal/diag"
)

type dkp struct {
  domain     string
  key        string
  wantPrefix string
}

func checkPresent(t *testing.T, d, k, pref string) {
  t.Helper()
  info, ok := diag.LookupFull(d, k)
  if !ok {
    t.Fatalf("missing catalog entry for %s.%s", d, k)
  }
  id := strings.TrimSpace(info.Entry.ID)
  if id == "" {
    t.Fatalf("empty ID for %s.%s", d, k)
  }
  if pref != "" && !strings.HasPrefix(id, pref) {
    t.Fatalf("ID %q for %s.%s does not start with %q", id, d, k, pref)
  }
  if strings.TrimSpace(info.Entry.Title) == "" {
    t.Fatalf("empty Title for %s.%s", d, k)
  }
}

// small helper: does at least one key in this domain exist?
func domainExists(domain string) bool {
  // Try a benign probe for each known domain we already assert elsewhere.
  switch domain {
  case "lexer":
    _, ok := diag.LookupFull("lexer", "generic_lexer_error")
    return ok
  case "parser":
    _, ok := diag.LookupFull("parser", "unexpected_token")
    return ok
  case "type":
    _, ok := diag.LookupFull("type", "undefined_name")
    return ok
  case "module":
    _, ok := diag.LookupFull("module", "import_cycle")
    return ok
  case "warn":
    _, ok := diag.LookupFull("warn", "unused_variable")
    return ok
  case "codegen":
    // probe one codegen key; skip block if domain not wired yet
    _, ok := diag.LookupFull("codegen", "c_backend_generic")
    return ok
  default:
    return false
  }
}

func TestCatalog_PresenceAndPrefixes(t *testing.T) {
  var cases []dkp

  // lexer
  cases = append(cases,
    dkp{"lexer", "unterminated_string", "DLE"},
    dkp{"lexer", "generic_lexer_error", "DLE"},
  )

  // parser
  cases = append(cases,
    dkp{"parser", "unexpected_token", "DPE"},
    dkp{"parser", "expected_token", "DPE"},
    dkp{"parser", "unclosed_delimiter", "DPE"},
    dkp{"parser", "trailing_or_extra_token", "DPE"},
    dkp{"parser", "invalid_assignment_target", "DPE"},
    dkp{"parser", "unexpected_after_pub", "DPE"},
    dkp{"parser", "wildcard_has_payload", "DPE"},
    dkp{"parser", "async_before_def", "DPE"},
    dkp{"parser", "await_requires_expr", "DPE"},
  )

  // type
  cases = append(cases,
    dkp{"type", "undefined_name", "DTE"},
    dkp{"type", "arity_mismatch", "DTE"},
    dkp{"type", "redeclared_symbol", "DTE"},
    dkp{"type", "type_mismatch", "DTE"},
    dkp{"type", "wrong_return_kind", "DTE"},
    dkp{"type", "assign_to_immutable", "DTE"},
    dkp{"type", "not_public", "DTE"},
    dkp{"type", "public_const_not_const", "DTE"},
    dkp{"type", "pub_let_mut_forbidden", "DTE"},
    dkp{"type", "use_none_not_void", "DTE"},
    dkp{"type", "reserved_identifier", "DTE"},
    dkp{"type", "shadow_builtin", "DTE"},
    dkp{"type", "import_name_conflict", "DTE"},
    dkp{"type", "use_none_instead_of_void", "DTE"},
    dkp{"type", "unsupported_assignment_target", "DTE"},
    dkp{"type", "unknown_struct_type", "DTE"},
    dkp{"type", "field_access_on_non_struct", "DTE"},
    dkp{"type", "cannot_assign_field_on_non_struct", "DTE"},
    dkp{"type", "illegal_defer_position", "DTE"},
    dkp{"type", "defer_expects_call", "DTE"},
    dkp{"type", "bad_condition_type", "DTE"},
    dkp{"type", "symbol_not_value", "DTE"},
    dkp{"type", "unknown_symbol_in_module_alias", "DTE"},
    dkp{"type", "unknown_field_on_struct", "DTE"},
    dkp{"type", "unknown_enum_variant", "DTE"},
    dkp{"type", "duplicate_match_arm", "DTE"},
    dkp{"type", "payloadless_variant_binder", "DTE"},
    dkp{"type", "await_outside_async", "DTE"},
    dkp{"type", "await_non_future", "DTE"},
    dkp{"type", "builtin_wrong_arity", "DTE"},
    dkp{"type", "builtin_arg_void", "DTE"},
    dkp{"type", "builtin_arg_unsupported_kind", "DTE"},
    dkp{"type", "call_wrong_arity", "DTE"},
    dkp{"type", "module_call_wrong_arity", "DTE"},
    dkp{"type", "enum_ctor_wrong_arity", "DTE"},
    dkp{"type", "incompatible_struct_assignment", "DTE"},
    dkp{"type", "incompatible_enum_assignment", "DTE"},
  )

  // module
  cases = append(cases,
    dkp{"module", "import_cycle", "DME"},
    dkp{"module", "not_found", "DME"},
    dkp{"module", "bad_import", "DME"},
    dkp{"module", "io_read", "DME"},
    dkp{"module", "parse_failed", "DME"},
    dkp{"module", "bad_entry", "DME"},
    dkp{"module", "entry_not_found", "DME"},
    dkp{"module", "internal", "DME"},
    dkp{"module", "duplicate_import", "DMW"},
  )

  // warn
  cases = append(cases,
    dkp{"warn", "unused_variable", "DW"},
    dkp{"warn", "shadowed_variable", "DW"},
    dkp{"warn", "unreachable_code", "DW"},
    dkp{"warn", "missing_explicit_return", "DW"},
    dkp{"warn", "non_exhaustive_match", "DW"},
  )

  // codegen – only enforce when the domain is wired
  if domainExists("codegen") {
    cases = append(cases,
      dkp{"codegen", "c_backend_generic", "DCE"},
      dkp{"codegen", "unsupported_return_type", "DCE"},
      dkp{"codegen", "unknown_return_type", "DCE"},
      dkp{"codegen", "non_async_returns_future", "DCE"},
      dkp{"codegen", "param_none_not_allowed", "DCE"},
      dkp{"codegen", "param_future_not_supported", "DCE"},
      dkp{"codegen", "unsupported_param_type", "DCE"},
      dkp{"codegen", "unknown_param_type", "DCE"},
      dkp{"codegen", "field_none_not_allowed", "DCE"},
      dkp{"codegen", "field_future_not_supported", "DCE"},
      dkp{"codegen", "unsupported_field_type", "DCE"},
      dkp{"codegen", "unknown_field_type", "DCE"},
      dkp{"codegen", "enum_payload_future_not_supported", "DCE"},
      dkp{"codegen", "enum_payload_unsupported", "DCE"},
      dkp{"codegen", "enum_payload_unknown", "DCE"},
      dkp{"codegen", "type_alias_future_not_supported", "DCE"},
      dkp{"codegen", "type_alias_unsupported", "DCE"},
      dkp{"codegen", "type_alias_unknown", "DCE"},
    )
  }

  for _, c := range cases {
    checkPresent(t, c.domain, c.key, c.wantPrefix)
  }
}
