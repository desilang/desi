package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// fromTypeName resolves an AST TypeName to a types.T, handling parameterized types.
// Returns nil if the type cannot be resolved.
func fromTypeName(tn *ast.TypeName) types.T {
	if tn == nil {
		return nil
	}

	// Handle parameterized types
	if len(tn.Params) > 0 {
		switch tn.Name {
		case "tuple":
			var elems []types.T
			for _, p := range tn.Params {
				t := fromTypeName(p)
				if t == nil {
					return nil
				}
				elems = append(elems, t)
			}
			return types.TupleOf(elems...)

		case "list":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := fromTypeName(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.ListOf(elem)

		case "set":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := fromTypeName(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.SetOf(elem)

		case "dict":
			if len(tn.Params) != 2 {
				return nil
			}
			key := fromTypeName(tn.Params[0])
			val := fromTypeName(tn.Params[1])
			if key == nil || val == nil {
				return nil
			}
			return types.DictOf(key, val)

		case "future":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := fromTypeName(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.FutureOf(elem)

		case "cptr":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := fromTypeName(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.CPtrOf(elem)
		}
		return nil
	}

	// No params - try simple name resolution
	if t, ok := types.FromName(tn.Name); ok {
		return t
	}

	return nil
}
