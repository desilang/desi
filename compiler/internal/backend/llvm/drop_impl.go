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

	// Check if variable was moved
	if m.currentMoves != nil && m.currentMoves[varName.Name] {
		return // Skip drop for moved variable
	}

	t, ok := x.Type.(types.T)
	if !ok {
		return
	}

	if !isHeapType(t) {
		return
	}

	// Get the actual value to free (handles SSA lookup automatically)
	_, ptrValue := m.operand(varName)

	// If the returned value starts with %%, it's a variable reference
	// We need to load it (mutable variable case)
	var ptrToFree string
	if len(ptrValue) > 0 && ptrValue[0] == '%' && m.ssa[varName.Name] == nil {
		// Mutable variable case: need to load from alloca
		loadTemp := fmt.Sprintf("%%drop_load_%d", m.tempID)
		m.tempID++
		wprintf(&m.funcs, "  %s = load ptr, ptr %s\n", loadTemp, ptrValue)
		ptrToFree = loadTemp
	} else {
		// SSA value or immediate
		ptrToFree = ptrValue
	}

	m.emitDropForType(ptrToFree, t)
}

// emitDropForType generates cleanup code for a value of a specific type
func (m *Module) emitDropForType(val string, t types.T) {
	// Check for null before dropping
	// if (val == null) return;
	cond := fmt.Sprintf("%%drop_cond_%d", m.tempID)
	m.tempID++
	wprintf(&m.funcs, "  %s = icmp eq ptr %s, null\n", cond, val)

	doneLabel := fmt.Sprintf("drop_done_%d", m.mergeID)
	doLabel := fmt.Sprintf("drop_do_%d", m.mergeID)
	m.mergeID++

	wprintf(&m.funcs, "  br i1 %s, label %%%s, label %%%s\n\n", cond, doneLabel, doLabel)
	wprintf(&m.funcs, "%s:\n", doLabel)

	switch t := t.(type) {
	case *types.Struct:
		m.emitStructDrop(val, t)
	case *types.Enum:
		m.emitEnumDrop(val, t)
	case *types.List:
		// Call list_free for proper cleanup
		wprintf(&m.funcs, "  call void @list_free(ptr %s)\n", val)
		m.ensureDecl("declare void @list_free(ptr)")
	case *types.Set:
		// Call set_free for proper cleanup
		wprintf(&m.funcs, "  call void @set_free(ptr %s)\n", val)
		m.ensureDecl("declare void @set_free(ptr)")
	case *types.Dict:
		// Call dict_free for proper cleanup
		wprintf(&m.funcs, "  call void @dict_free(ptr %s)\n", val)
		m.ensureDecl("declare void @dict_free(ptr)")
	default:
		// Simple free for other heap types (str, etc)
		wprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
		m.ensureDecl("declare void @free(ptr)")
	}

	wprintf(&m.funcs, "  br label %%%s\n\n", doneLabel)
	wprintf(&m.funcs, "%s:\n", doneLabel)
}

// emitStructDrop generates cleanup code for a struct value
func (m *Module) emitStructDrop(val string, st *types.Struct) {
	// Recursively drop heap-allocated fields using offset-based GEP
	for i, field := range st.Fields {
		if !isHeapType(field.Type) {
			continue
		}

		// Calculate byte offset for this field
		offset := calculateFieldOffset(st, i)

		// GEP to field at byte offset (works with opaque pointers)
		fieldPtr := fmt.Sprintf("%%field_ptr_%d_%d", m.tempID, i)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = getelementptr i8, ptr %s, i32 %d\n",
			fieldPtr, val, offset)

		// Load field value (ptr type)
		fieldVal := fmt.Sprintf("%%field_val_%d_%d", m.tempID, i)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = load ptr, ptr %s\n", fieldVal, fieldPtr)

		// Recursively drop field
		m.emitDropForType(fieldVal, field.Type)
	}

	// Free the struct itself
	fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
	m.ensureDecl("declare void @free(ptr)")
	m.ensureDecl("declare i32 @printf(ptr, ...)")
}

// emitEnumDrop generates cleanup code for an enum value
func (m *Module) emitEnumDrop(val string, et *types.Enum) {
	// TODO: Load tag, switch on variants, recursively drop payloads
	// For now, just free the enum pointer itself to avoid the main leak

	fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
	m.ensureDecl("declare void @free(ptr)")
}

func isHeapType(t types.T) bool {
	switch t.(type) {
	case *types.Struct, *types.Enum, *types.List, *types.Dict, *types.Set:
		return true
	}
	return false
}
