package check

import (
	"strings"

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

		case "rc":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.RcOf(elem)

		case "arc":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.ArcOf(elem)

		case "weak":
			if len(tn.Params) != 1 {
				return nil
			}
			elem := c.resolveType(tn.Params[0])
			if elem == nil {
				return nil
			}
			return types.WeakOf(elem)
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

				// Handle list type aliases (e.g., type MyList<T> = list<T>)
				if listType, ok := sym.Type.(*types.List); ok {
					if _, ok := listType.Elem.(*types.TypeParam); ok && len(args) > 0 {
						// TypeParam in elem position - substitute with first type arg
						return &types.List{Elem: args[0]}
					}
					// If elem is already concrete or no args, return as-is
					return sym.Type
				}

				// Handle set type aliases (e.g., type MySet<T> = set<T>)
				if setType, ok := sym.Type.(*types.Set); ok {
					if _, ok := setType.Elem.(*types.TypeParam); ok {
						if len(args) > 0 {
							return &types.Set{Elem: args[0]}
						}
					}
					return sym.Type
				}

				// Handle dict type aliases (e.g., type MyDict<K, V> = dict<K, V>)
				if dictType, ok := sym.Type.(*types.Dict); ok {
					newKey := dictType.Key
					newVal := dictType.Val
					argIdx := 0
					if tp, ok := dictType.Key.(*types.TypeParam); ok {
						if argIdx < len(args) {
							newKey = args[argIdx]
							argIdx++
						}
						_ = tp // use tp to avoid unused warning
					}
					if tp, ok := dictType.Val.(*types.TypeParam); ok {
						if argIdx < len(args) {
							newVal = args[argIdx]
						}
						_ = tp
					}
					if newKey != dictType.Key || newVal != dictType.Val {
						return &types.Dict{Key: newKey, Val: newVal}
					}
					return sym.Type
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

	// Handle qualified type names: module.TypeName (e.g., "http.Request")
	if strings.Contains(tn.Name, ".") {
		parts := strings.SplitN(tn.Name, ".", 2)
		modName, typeName := parts[0], parts[1]
		if c.info != nil && c.info.R != nil && c.info.R.ModuleExports != nil {
			for mpath, ex := range c.info.R.ModuleExports {
				// Match module path ending (e.g., "http" matches ".../http")
				if ex != nil && ex.TypeAliases != nil {
					pathParts := strings.Split(mpath, "/")
					lastPart := pathParts[len(pathParts)-1]
					if lastPart == modName || mpath == modName {
						if t, ok := ex.TypeAliases[typeName]; ok {
							return t
						}
					}
				}
			}
		}
	}

	return nil
}
