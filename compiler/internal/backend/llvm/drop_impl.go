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
		fmt.Fprintf(&m.funcs, "  %s = load ptr, ptr %s\n", loadTemp, ptrValue)
		ptrToFree = loadTemp
	} else {
		// SSA value case: use directly
		ptrToFree = ptrValue
	}

	m.emitDropForType(ptrToFree, t)
}

// emitDropForType recursively drops a value of type t
func (m *Module) emitDropForType(val string, t types.T) {
	// Null check
	cond := fmt.Sprintf("%%drop_cond_%d", m.tempID)
	m.tempID++
	fmt.Fprintf(&m.funcs, "  %s = icmp eq ptr %s, null\n", cond, val)

	dropLabel := fmt.Sprintf("drop_do_%d", m.mergeID)
	doneLabel := fmt.Sprintf("drop_done_%d", m.mergeID)
	m.mergeID++

	fmt.Fprintf(&m.funcs, "  br i1 %s, label %%%s, label %%%s\n", cond, doneLabel, dropLabel)

	fmt.Fprintf(&m.funcs, "\n%s:\n", dropLabel)
	if st, ok := t.(*types.Struct); ok {
		m.emitStructDrop(val, st)
	} else if et, ok := t.(*types.Enum); ok {
		m.emitEnumDrop(val, et)
	}
	fmt.Fprintf(&m.funcs, "  br label %%%s\n", doneLabel)

	fmt.Fprintf(&m.funcs, "\n%s:\n", doneLabel)
}

// emitStructDrop generates cleanup code for a struct value
func (m *Module) emitStructDrop(val string, st *types.Struct) {
	// TODO: Recursive field dropping requires typed pointers or explicit offset calculation
	// For now, just free the struct pointer itself to prevent the main leak
	// This prevents the struct allocation from leaking
	fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
	m.ensureDecl("declare void @free(ptr)")
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
