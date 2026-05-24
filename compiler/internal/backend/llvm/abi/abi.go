package abi

// Info holds platform-specific ABI information for LLVM code generation.
type Info struct {
	TargetTriple string
	TargetLayout string
	VariadicConv CallConvention
}

// CallConvention describes how variadic arguments are passed.
type CallConvention int

const (
	// StackBased indicates variadic arguments are passed on the stack (ARM64 macOS/Linux).
	StackBased CallConvention = iota
	// RegisterBased indicates variadic arguments are passed in registers (x86_64).
	RegisterBased
)

var current *Info

// Current returns the ABI info for the current platform.
// This is set at init time by the platform-specific file.
func Current() *Info {
	if current == nil {
		panic("abi: no platform-specific ABI registered")
	}
	return current
}

// Register sets the current platform's ABI info.
// Called by platform-specific files in their init() functions.
func Register(info *Info) {
	current = info
}

// IsWindows returns true when targeting Windows MSVC.
// Used to emit platform-specific IR patterns (e.g., 2-arg _setjmp).
func (i *Info) IsWindows() bool {
	// MSVC targets have "windows-msvc" in the triple
	return len(i.TargetTriple) > 0 &&
		(contains(i.TargetTriple, "windows-msvc") || contains(i.TargetTriple, "windows"))
}

// contains is a simple string.Contains replacement to avoid importing strings.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// NeedsI32ToI64Promotion returns true if i32 arguments to variadic functions
// should be promoted to i64 for correct stack alignment.
func (i *Info) NeedsI32ToI64Promotion(fnName string) bool {
	// Only needed for stack-based calling conventions
	if i.VariadicConv != StackBased {
		return false
	}
	// Apply to asprintf and printf
	return fnName == "asprintf" || fnName == "printf"
}

// VariadicSignature returns the explicit LLVM IR function signature for variadic functions.
// This is critical for platforms with stack-based calling conventions.
func (i *Info) VariadicSignature(fnName string, retType string) string {
	switch fnName {
	case "asprintf":
		return "declare i32 @asprintf(ptr, ptr, ...)"
	case "printf":
		return "declare i32 @printf(ptr, ...)"
	default:
		// Generic variadic declaration
		return ""
	}
}

// ExplicitCallSyntax returns the call instruction syntax with explicit signature if needed.
// For stack-based conventions, we need to include the signature in the call instruction.
func (i *Info) ExplicitCallSyntax(fnName string, retType string) string {
	if i.VariadicConv != StackBased {
		return "" // No explicit syntax needed
	}

	switch fnName {
	case "asprintf":
		return "(ptr, ptr, ...)"
	case "printf":
		return "(ptr, ...)"
	default:
		return ""
	}
}
