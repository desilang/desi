package llvm

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// emitDrop generates cleanup code for a value based on its type
func (m *Module) emitDrop(x *hir.Drop) {
	// If no type information, can't do anything
	if x.Type == nil {
		return
	}

	varName, ok := x.Val.(hir.Var)
	if !ok {
		return // Can only drop variables
	}

	t, ok := x.Type.(types.T)
	if !ok {
		return
	}

	// Only free heap-allocated types (Enums and Structs)
	// Primitives (int, bool, str, etc.) don't need cleanup
	var needsCleanup bool

	if _, ok := t.(*types.Enum); ok {
		needsCleanup = true
	} else if _, ok := t.(*types.Struct); ok {
		needsCleanup = true
	}

	if !needsCleanup {
		return
	}

	// Get the actual value to free (handles SSA lookup automatically)
	// The operand() method will:
	// 1. Check SSA map for the variable
	// 2. Return the actual heap pointer (e.g., %call3)
	// 3. For mutable variables, return %%%varName which we'll need to load
	_, ptrValue := m.operand(varName)

	// If the returned value starts with %%, it's a variable reference
	// We need to load it (mutable variable case)
	var ptrToFree string
	if len(ptrValue) > 0 && ptrValue[0] == '%' && m.ssa[varName.Name] == nil {
		// Mutable variable case: need to load from alloca
		loadTemp := fmt.Sprintf("%%drop_load_%d", m.tempID)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = load ptr, ptr %s\n", loadTemp, ptrValue)
		ptrToFree = loadTemp
	} else {
		// SSA value case: use directly
		ptrToFree = ptrValue
	}

	// Generate free call
	fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", ptrToFree)
	m.ensureDecl("declare void @free(ptr)")
}

// emitEnumDrop generates cleanup code for an enum value
func (m *Module) emitEnumDrop(varName string, enumType *types.Enum) {
	// Enums are heap-allocated pointers
	// Layout: i32 tag at offset 0, ptr payload at offset 4

	// TODO: Free payloads for variants with fields
	// For now, we only free the enum struct itself to avoid crashes
	// from trying to free unallocated payloads (variants with no fields)
	// This still prevents the main leak (the enum struct)

	// Free the enum struct itself
	fmt.Fprintf(&m.funcs, "  call void @free(ptr %%%s)\n", varName)
	m.ensureDecl("declare void @free(ptr)")
}

// emitStructDrop generates cleanup code for a struct value
func (m *Module) emitStructDrop(varName string, structType *types.Struct) {
	// Structs are heap-allocated pointers
	// For now, just free the struct pointer
	// TODO: Recursively drop fields if they contain heap-allocated types

	fmt.Fprintf(&m.funcs, "  call void @free(ptr %%%s)\n", varName)
	m.ensureDecl("declare void @free(ptr)")
}
