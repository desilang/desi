package llvm

type funcSig struct {
	ret    string
	params []string
}

var funcSigOverrides = map[string]funcSig{}

// SetFuncSig registers a textual LLVM signature override.
// Use empty strings to keep defaults for ret or any param.
func SetFuncSig(name, ret string, params []string) {
	cp := make([]string, len(params))
	copy(cp, params)
	funcSigOverrides[name] = funcSig{ret: ret, params: cp}
}

func getFuncSig(name string) (funcSig, bool) {
	s, ok := funcSigOverrides[name]
	return s, ok
}

func init() {
	// Dict runtime overrides
	SetFuncSig("dict_new", "ptr", nil)
	SetFuncSig("dict_get", "ptr", nil)
	SetFuncSig("dict_keys", "ptr", nil)
	SetFuncSig("dict_values", "ptr", nil)
	SetFuncSig("dict_clear", "void", nil)
	SetFuncSig("dict_values", "ptr", nil)
	SetFuncSig("dict_clear", "void", nil)
	SetFuncSig("dict_free", "void", nil)

	// Set runtime overrides
	SetFuncSig("set_new", "ptr", nil)
	SetFuncSig("set_add", "void", nil)
	SetFuncSig("set_remove", "void", nil)
	SetFuncSig("set_contains", "i1", nil)
	SetFuncSig("set_clear", "void", nil)
	SetFuncSig("set_free", "void", nil)
	SetFuncSig("set_to_array", "ptr", nil)
	SetFuncSig("set_union", "ptr", nil)
	SetFuncSig("set_intersection", "ptr", nil)
	SetFuncSig("set_difference", "ptr", nil)
	SetFuncSig("bool_to_cstring", "ptr", nil)

	// List runtime overrides
	SetFuncSig("list_new", "ptr", nil)
	SetFuncSig("list_append", "void", nil)
	SetFuncSig("list_get", "ptr", nil)
	SetFuncSig("list_set", "void", nil)
	SetFuncSig("list_len", "i64", nil)
	SetFuncSig("list_slice", "ptr", nil)
	SetFuncSig("list_free", "void", nil)
	SetFuncSig("list_copy", "ptr", nil)
	SetFuncSig("list_extend", "void", nil)
	SetFuncSig("list_insert", "void", nil)
	SetFuncSig("list_pop", "ptr", nil)
	SetFuncSig("list_remove", "void", nil)
	SetFuncSig("list_reverse", "void", nil)
	SetFuncSig("list_index", "i64", nil)
	SetFuncSig("list_count", "i64", nil)
	SetFuncSig("list_contains", "i1", nil)
	SetFuncSig("list_map", "ptr", nil)
	SetFuncSig("list_filter", "ptr", nil)
	SetFuncSig("list_reduce", "ptr", nil)
	SetFuncSig("list_any", "i1", nil)
	SetFuncSig("list_all", "i1", nil)
}
