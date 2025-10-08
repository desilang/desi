package diag_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
)

type dk struct{ domain, key, wantPrefix string }

// assert presence and ID prefix (DLE/DPE/DTE/DME/DMW/DCE).
func check(t *testing.T, d, k, pref string) {
	t.Helper()
	info, ok := diag.LookupFull(d, k)
	if !ok {
		t.Fatalf("missing catalog entry for %s.%s", d, k)
	}
	if id := strings.TrimSpace(info.Entry.ID); id == "" {
		t.Fatalf("empty ID for %s.%s", d, k)
	} else if pref != "" && !strings.HasPrefix(id, pref) {
		t.Fatalf("ID %q for %s.%s does not start with %q", id, d, k, pref)
	}
	if strings.TrimSpace(info.Entry.Title) == "" {
		t.Fatalf("empty Title for %s.%s", d, k)
	}
}

func TestCatalog_PresenceAndPrefixes(t *testing.T) {
	var cases []dk

	// lexer
	cases = append(cases,
		dk{"lexer", "unterminated_string", "DLE"},
		dk{"lexer", "generic_lexer_error", "DLE"},
	)

	// parser (pdiag + async/await)
	cases = append(cases,
		dk{"parser", "unexpected_token", "DPE"},
		dk{"parser", "expected_token", "DPE"},
		dk{"parser", "unclosed_delimiter", "DPE"},
		dk{"parser", "trailing_or_extra_token", "DPE"},
		dk{"parser", "invalid_assignment_target", "DPE"},
		dk{"parser", "unexpected_after_pub", "DPE"},
		dk{"parser", "wildcard_has_payload", "DPE"},
		dk{"parser", "async_before_def", "DPE"},
		dk{"parser", "await_requires_expr", "DPE"},
	)

	// type (diagshim.go wrappers)
	cases = append(cases,
		dk{"type", "undefined_name", "DTE"},
		dk{"type", "arity_mismatch", "DTE"},
		dk{"type", "redeclared_symbol", "DTE"},
		dk{"type", "type_mismatch", "DTE"},
		dk{"type", "wrong_return_kind", "DTE"},
		dk{"type", "assign_to_immutable", "DTE"},
		dk{"type", "not_public", "DTE"},
		dk{"type", "public_const_not_const", "DTE"},
		dk{"type", "pub_let_mut_forbidden", "DTE"},
		dk{"type", "use_none_not_void", "DTE"},
		dk{"type", "reserved_identifier", "DTE"},
		dk{"type", "shadow_builtin", "DTE"},
		dk{"type", "import_name_conflict", "DTE"},
		dk{"type", "use_none_instead_of_void", "DTE"},
		dk{"type", "unsupported_assignment_target", "DTE"},
		dk{"type", "unknown_struct_type", "DTE"},
		dk{"type", "field_access_on_non_struct", "DTE"},
		dk{"type", "cannot_assign_field_on_non_struct", "DTE"},
		dk{"type", "illegal_defer_position", "DTE"},
		dk{"type", "defer_expects_call", "DTE"},
		dk{"type", "bad_condition_type", "DTE"},
		dk{"type", "symbol_not_value", "DTE"},
		dk{"type", "unknown_symbol_in_module_alias", "DTE"},
		dk{"type", "unknown_field_on_struct", "DTE"},
		dk{"type", "unknown_enum_variant", "DTE"},
		dk{"type", "duplicate_match_arm", "DTE"},
		dk{"type", "payloadless_variant_binder", "DTE"},
		dk{"type", "await_outside_async", "DTE"},
		dk{"type", "await_non_future", "DTE"},
		dk{"type", "builtin_wrong_arity", "DTE"},
		dk{"type", "builtin_arg_void", "DTE"},
		dk{"type", "builtin_arg_unsupported_kind", "DTE"},
		dk{"type", "call_wrong_arity", "DTE"},
		dk{"type", "module_call_wrong_arity", "DTE"},
		dk{"type", "enum_ctor_wrong_arity", "DTE"},
		dk{"type", "incompatible_struct_assignment", "DTE"},
		dk{"type", "incompatible_enum_assignment", "DTE"},
	)

	// module (resolver) – errors + warnings
	cases = append(cases,
		dk{"module", "import_cycle", "DME"},
		dk{"module", "not_found", "DME"},
		dk{"module", "bad_import", "DME"},
		dk{"module", "io_read", "DME"},
		dk{"module", "parse_failed", "DME"},
		dk{"module", "bad_entry", "DME"},
		dk{"module", "entry_not_found", "DME"},
		dk{"module", "internal", "DME"},
		dk{"module", "duplicate_import", "DMW"},
		dk{"module", "import_self", "DMW"},
		dk{"module", "import_alias_conflict", "DMW"},
	)

	// warn (checker warnings)
	cases = append(cases,
		dk{"warn", "unused_variable", "DW"},
		dk{"warn", "shadowed_variable", "DW"},
		dk{"warn", "unreachable_code", "DW"},
		dk{"warn", "missing_explicit_return", "DW"},
		dk{"warn", "non_exhaustive_match", "DW"},
	)

	// codegen/c
	cases = append(cases,
		dk{"codegen", "c_backend_generic", "DCE"},
		dk{"codegen", "unsupported_return_type", "DCE"},
		dk{"codegen", "unknown_return_type", "DCE"},
		dk{"codegen", "non_async_returns_future", "DCE"},
		dk{"codegen", "param_none_not_allowed", "DCE"},
		dk{"codegen", "param_future_not_supported", "DCE"},
		dk{"codegen", "unsupported_param_type", "DCE"},
		dk{"codegen", "unknown_param_type", "DCE"},
		dk{"codegen", "field_none_not_allowed", "DCE"},
		dk{"codegen", "field_future_not_supported", "DCE"},
		dk{"codegen", "unsupported_field_type", "DCE"},
		dk{"codegen", "unknown_field_type", "DCE"},
		dk{"codegen", "enum_payload_future_not_supported", "DCE"},
		dk{"codegen", "enum_payload_unsupported", "DCE"},
		dk{"codegen", "enum_payload_unknown", "DCE"},
		dk{"codegen", "type_alias_future_not_supported", "DCE"},
		dk{"codegen", "type_alias_unsupported", "DCE"},
		dk{"codegen", "type_alias_unknown", "DCE"},
	)

	for _, c := range cases {
		check(t, c.domain, c.key, c.wantPrefix)
	}
}
