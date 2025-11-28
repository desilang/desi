package llvm

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// emitDrop generates cleanup code for a value based on its type
func (m *Module) emitDrop(x *hir.Drop) {
	// TODO: Implement proper memory management
	// Current issue: we're trying to free stack-allocated variables (%opt)
	// instead of the heap-allocated values they hold (%call3).
	// Need to rethink how we track the actual heap pointers vs stack slots.
	// For now, leaving as no-op to prevent crashes.
	_ = x // Suppress unused warning
	return
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
