package llvm

import (
	"github.com/desilang/desi/compiler/internal/types"
)

// calculateFieldOffset calculates the byte offset of a field in a struct
// Returns the offset in bytes for the given field index
func calculateFieldOffset(st *types.Struct, fieldIndex int) int {
	offset := 0
	for i := 0; i < fieldIndex; i++ {
		offset += fieldSize(st.Fields[i].Type)
	}
	return offset
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
