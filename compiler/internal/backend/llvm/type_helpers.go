package llvm

import "github.com/desilang/desi/compiler/internal/types"

// isReferenceType checks if a type is a reference type that shouldn't be allocated.
// Reference types (set, dict, list, str) are already pointers from method calls.
func isReferenceType(t interface{}) bool {
	if t == nil {
		return false
	}

	// Type assertion to types.T
	typ, ok := t.(types.T)
	if !ok {
		return false
	}

	// Check if it's a reference type (including classes which are heap-allocated)
	switch typ.(type) {
	case *types.Set, *types.Dict, *types.List, *types.Arena, *types.Class:
		return true
	}

	// Check if it's a string (Str is a *basic variable)
	if typ == types.Str {
		return true
	}

	return false
}
