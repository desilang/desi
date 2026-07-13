package llvm

import (
	"github.com/desilang/desi/compiler/internal/types"
)

// calculateFieldOffset calculates the byte offset of a field in a struct.
// MUST mirror the construction layout (lowerer's getSize/getAlign walk):
// each field's offset is aligned to the field's natural alignment. Summing
// raw sizes reads the wrong slot — e.g. {id: int, status: Enum} stores the
// ptr at offset 8 (padded), not 4 — and the drop then frees garbage.
func calculateFieldOffset(st *types.Struct, fieldIndex int) int {
	offset := 0
	for i := 0; i <= fieldIndex; i++ {
		align := fieldAlign(st.Fields[i].Type)
		if align > 0 && offset%align != 0 {
			offset += align - offset%align
		}
		if i == fieldIndex {
			return offset
		}
		offset += fieldSize(st.Fields[i].Type)
	}
	return offset
}

// fieldAlign mirrors the lowerer's getAlign (module_lower.go).
func fieldAlign(t types.T) int {
	if t == nil {
		return 8
	}
	switch t.(type) {
	case *types.Struct, *types.Enum, *types.List, *types.Dict, *types.Set:
		return 8 // ptr
	}
	switch t.String() {
	case "int", "i32", "u32", "f32":
		return 4
	case "i64", "u64", "isize", "usize", "float", "f64":
		return 8
	case "bool":
		return 1
	case "str":
		return 8
	default:
		return 8
	}
}

// fieldSize returns the size in bytes of a type
func fieldSize(t types.T) int {
	switch t.(type) {
	case *types.Struct, *types.Enum, *types.List, *types.Dict, *types.Set:
		return 8 // ptr on 64-bit
	default:
		// Check basic types
		switch t.String() {
		case "int":
			return 4 // i32
		case "float":
			return 8 // double
		case "bool":
			return 1 // i1
		case "str":
			return 8 // ptr
		case "none":
			return 8 // ptr
		default:
			return 8 // default ptr size
		}
	}
}
