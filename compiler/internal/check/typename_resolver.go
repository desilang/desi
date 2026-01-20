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

	// Handle tuple types (T1, T2)
	if len(tn.TupleTypes) > 0 {
		var elems []types.T
		for _, et := range tn.TupleTypes {
			t := c.resolveType(et)
			if t == nil {
				return nil
			}
			elems = append(elems, t)
		}
		return types.TupleOf(elems...)
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

		case "cptr", "ptr":
			// ptr[T] or cptr[T] - typed pointer
			// ptr or cptr alone is invalid in this branch (has params)
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.CPtrOf(elem)

		case "Option":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.OptionOf(elem)

		case "Result":
			if len(tn.Params) != 2 {
				return nil
			}
			ok := c.resolveType(tn.Params[0])
			err := c.resolveType(tn.Params[1])
			if ok == nil || err == nil {
				return nil
			}
			return types.ResultOf(ok, err)
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

				// Handle tuple type aliases (e.g., type Pair<A, B> = (A, B))
				// We need to substitute type params inline since Tuple isn't a GenericBase
				if tupType, ok := sym.Type.(*types.Tuple); ok {
					// Build substitution map from TypeParams to concrete args
					subst := make(map[string]types.T)
					// Find TypeParams in tuple elements and map to args
					paramIdx := 0
					for _, elem := range tupType.Elems {
						if tp, ok := elem.(*types.TypeParam); ok {
							if paramIdx < len(args) {
								subst[tp.Name] = args[paramIdx]
								paramIdx++
							}
						}
					}
					// Substitute all elements
					newElems := make([]types.T, len(tupType.Elems))
					for i, elem := range tupType.Elems {
						if tp, ok := elem.(*types.TypeParam); ok {
							if sub, found := subst[tp.Name]; found {
								newElems[i] = sub
							} else {
								newElems[i] = elem
							}
						} else {
							newElems[i] = elem
						}
					}
					return types.TupleOf(newElems...)
				}

				// Create Generic instantiation for other generic types
				// First, validate that type arguments satisfy bounds
				c.validateGenericBounds(sym.Type, args, tn.Span)

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
		// Also check SymFunc with classType for imported nested classes
		if sym != nil && sym.Kind == SymFunc && sym.Type != nil {
			if _, isClass := sym.Type.(*types.Class); isClass {
				return sym.Type
			}
		}
	}

	return nil
}
