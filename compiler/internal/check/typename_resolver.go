package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// c.resolveType resolves an AST TypeName to a types.T, handling parameterized types.
// Returns nil if the type cannot be resolved.
// resolveType resolves an AST TypeName to a types.T, handling parameterized types.
// Returns nil if the type cannot be resolved.
func (c *checker) resolveType(tn *ast.TypeName) types.T {
	if tn == nil {
		return nil
	}

	// Handle union types (int|float|none)
	if len(tn.UnionTypes) > 0 {
		var variants []types.T
		for _, ut := range tn.UnionTypes {
			t := c.resolveType(ut)
			if t == nil {
				return nil // If any variant fails to resolve, fail the whole union
			}
			variants = append(variants, t)
		}
		return types.UnionOf(variants...)
	}

	// Handle parameterized types
	if len(tn.Params) > 0 {
		switch tn.Name {
		case "tuple":
			var elems []types.T
			for _, p := range tn.Params {
				t := c.resolveType(p)
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
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.ListOf(elem)

		case "set":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.SetOf(elem)

		case "dict":
			if len(tn.Params) != 2 {
				return nil
			}
			key := c.resolveType(tn.Params[0])
			val := c.resolveType(tn.Params[1])
			if key == nil || val == nil {
				return nil
			}
			return types.DictOf(key, val)

		case "future":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.FutureOf(elem)

		case "cptr":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.CPtrOf(elem)
		}

		// Check for user-defined generic types (enum/struct/class with type params)
		if c.scope != nil {
			sym := c.scope.Lookup(tn.Name)
			if sym != nil && sym.Kind == SymType {
				// Resolve type arguments
				var args []types.T
				for _, p := range tn.Params {
					arg := c.resolveType(p)
					if arg == nil {
						return nil
					}
					args = append(args, arg)
				}
				// Create Generic instantiation
				return &types.Generic{
					Base: sym.Type,
					Args: args,
				}
			}
		}
		return nil
	}

	// No params - try simple name resolution
	if t, ok := types.FromName(tn.Name); ok {
		return t
	}

	// Look up user-defined types (structs, classes, etc.)
	if c.scope != nil {
		sym := c.scope.Lookup(tn.Name)
		if sym != nil && sym.Kind == SymType {
			return sym.Type
		}
	}

	return nil
}
