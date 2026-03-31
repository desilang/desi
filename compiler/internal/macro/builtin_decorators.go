package macro

// builtinDecorators is the exhaustive list of Tier 1 (built-in) decorator names.
// These are hardcoded in the compiler and CANNOT be overridden by macro protocols.
//
// Tier 1 decorators have special handling in the compiler:
//   - @extern       → FFI: declares C function binding (check.go, hir_lower.go, module_lower.go)
//   - @ffi_struct   → FFI: C-compatible struct layout (check_type.go)
//   - @packed       → FFI: packed struct layout (check_type.go)
//   - @inline       → LLVM: marks function for inlining (hir_lower.go)
//   - @staticmethod → Class: no self parameter injection (class_lower.go)
//   - @classmethod  → Class: no self parameter injection (class_lower.go)
//   - @property     → Class: accessed without () (check_type.go)
//   - @abstract     → Class: must be overridden in subclass (check_type.go)
//   - @test         → Testing: marks function as test case (check.go)
var builtinDecorators = map[string]bool{
	"extern":       true,
	"ffi_struct":   true,
	"packed":       true,
	"inline":       true,
	"staticmethod": true,
	"classmethod":  true,
	"property":     true,
	"abstract":     true,
	"test":         true,
	"macro":        true, // @macro itself is a Tier 1 decorator (triggers macro loading)
}

// IsBuiltinDecorator returns true if the given name is a Tier 1 built-in decorator.
// Built-in decorators are handled by existing hardcoded compiler logic and
// MUST NOT be registered as macro protocols.
func IsBuiltinDecorator(name string) bool {
	return builtinDecorators[name]
}
