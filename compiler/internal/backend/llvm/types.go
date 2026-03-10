package llvm

import (
	"github.com/desilang/desi/compiler/internal/types"
)

// llvmType converts a types.T to its LLVM type representation
func llvmType(t types.T) string {
	switch t.(type) {
	case *types.Struct:
		// Struct types are represented as pointers in Desi
		return "ptr"
	case *types.Enum:
		// Enum types are represented as pointers in Desi
		return "ptr"
	case *types.List:
		return "ptr"
	case *types.Dict:
		return "ptr"
	case *types.Set:
		return "ptr"
	default:
		// Check for basic types by name
		switch t.String() {
		case "int":
			return "i32"
		case "float":
			return "double"
		case "bool":
			return "i1"
		case "str":
			return "ptr"
		case "bytes":
			return "ptr"
		case "none":
			return "ptr"
		default:
			// Default to ptr for unknown types
			return "ptr"
		}
	}
}

// emitStructTypeDefs emits LLVM type definitions for all structs
// This must be called before emitting functions that reference these types
func (m *Module) emitStructTypeDefs(structs []*types.Struct, enums []*types.Enum) {
	// Emit struct types
	for _, st := range structs {
		var fieldTypes []string
		for _, f := range st.Fields {
			fieldTypes = append(fieldTypes, llvmType(f.Type))
		}
		// %StructName = type { field1_type, field2_type, ... }
		if len(fieldTypes) == 0 {
			wprintf(&m.globals, "%%%s = type {}\n", st.Name)
		} else {
			wprintf(&m.globals, "%%%s = type { %s }\n", st.Name, joinTypes(fieldTypes))
		}
	}

	// Emit enum types
	// Enums have layout: { i32 tag, ptr payload }
	for _, et := range enums {
		wprintf(&m.globals, "%%%s = type { i32, ptr }\n", et.Name)
	}
}

func joinTypes(types []string) string {
	if len(types) == 0 {
		return ""
	}
	result := types[0]
	for i := 1; i < len(types); i++ {
		result += ", " + types[i]
	}
	return result
}
