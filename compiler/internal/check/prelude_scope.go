package check

// injectPreludeIntoScope makes prelude callable names (e.g., "print", "str")
// visible as identifiers in the top scope, so the checker doesn't raise
// DTE0001 before consulting Info.Funcs for overload resolution.
func injectPreludeIntoScope(top *Scope, info *Info) {
	if top == nil || info == nil {
		return
	}
	for name := range info.Funcs {
		// We don't need a concrete function type here; overloads live in Info.Funcs.
		// Define is idempotent per scope (we ignore false).
		_ = top.Define(&Symbol{Name: name, Kind: SymFunc})
	}
}

// preludeBuiltinNames enumerates builtin callables that cannot be shadowed at module scope.
var preludeBuiltinNames = map[string]struct{}{
	"print": {},
	"str":   {},
	"len":   {},
	"bool":  {},
	"open":  {},
}

// isPreludeBuiltinName reports whether 'name' is one of the always-available builtins.
func isPreludeBuiltinName(name string) bool {
	_, ok := preludeBuiltinNames[name]
	return ok
}
