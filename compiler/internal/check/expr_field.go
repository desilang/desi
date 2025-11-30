package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// typFieldExpr handles obj.field or obj.method.
// Currently supports:
// - dict methods: get, has_key, pop, clear, keys, values
func (c *checker) typFieldExpr(x *ast.FieldExpr) types.T {
	// Special case: EnumType.Variant (accessing variant constructor on enum type)
	// x.X might be an Ident referring to an enum type
	if id, ok := x.X.(*ast.Ident); ok {
		sym := c.scope.Lookup(id.Name)
		if sym != nil && sym.Kind == SymType {
			if enumDecl, ok := sym.Node.(*ast.EnumDecl); ok {
				// x is EnumType.Variant
				// Look up the enum type
				var enumType *types.Enum
				if sym.Type != nil {
					enumType, _ = sym.Type.(*types.Enum)
				}

				// Verify variant exists
				variantName := x.Name.Name
				var variantFound bool
				var variantParams []types.T

				if enumType != nil {
					for _, v := range enumType.Variants {
						if v.Name == variantName {
							variantFound = true
							// Variant constructors take payload fields as parameters
							for _, f := range v.Fields {
								variantParams = append(variantParams, f.Type)
							}
							break
						}
					}
				} else {
					// Fallback: check AST
					for _, v := range enumDecl.Variants {
						if v.Name.Name == variantName {
							variantFound = true
							// For MVP, each variant has at most one Type field
							if v.Type != nil {
								if t := c.resolveType(v.Type); t != nil {
									variantParams = append(variantParams, t)
								}
							}
							break
						}
					}
				}

				if !variantFound {
					c.add(diagAt("DTE0001", x.Name.Span, "undefined variant '"+variantName+"' on enum '"+id.Name+"'"))
					return nil
				}

				// Return a function type: (params...) -> EnumType
				// But we need the enum type
				resultType := sym.Type
				if resultType == nil && enumType != nil {
					resultType = enumType
				}

				funcType := types.FuncOf(variantParams, resultType, false)
				c.info.Types[x] = funcType
				return funcType
			}
		}
	}

	t := c.typ(x.X)
	if t == nil {
		return nil
	}

	// Handle Generic types (e.g. Option[int].Some)
	if gen, ok := t.(*types.Generic); ok {
		if enumT, ok := gen.Base.(*types.Enum); ok {
			// Look up variant
			variantName := x.Name.Name
			var variantFound bool
			var variantParams []types.T

			for _, v := range enumT.Variants {
				if v.Name == variantName {
					variantFound = true
					// Substitute type parameters
					// Map TypeParam name -> Generic Arg
					subst := make(map[string]types.T)
					if len(enumT.TypeParams) == len(gen.Args) {
						for i, tp := range enumT.TypeParams {
							subst[tp.Name] = gen.Args[i]
						}
					}

					for _, f := range v.Fields {
						// Perform substitution
						if tp, ok := f.Type.(*types.TypeParam); ok {
							if arg, found := subst[tp.Name]; found {
								variantParams = append(variantParams, arg)
							} else {
								variantParams = append(variantParams, f.Type)
							}
						} else {
							// TODO: Recursive substitution for complex types (e.g. List[T])
							// For now, assume simple T
							variantParams = append(variantParams, f.Type)
						}
					}
					break
				}
			}

			if !variantFound {
				c.add(diagAt("DTE0001", x.Name.Span, "undefined variant '"+variantName+"' on generic enum"))
				return nil
			}

			// Return function type: (params...) -> GenericType
			funcType := types.FuncOf(variantParams, gen, false)
			c.info.Types[x] = funcType
			return funcType
		}
	}

	// Handle Dict methods
	if d, ok := t.(*types.Dict); ok {
		return c.resolveDictMethod(x, d)
	}

	// Handle Set methods
	if s, ok := t.(*types.Set); ok {
		return c.resolveSetMethod(x, s)
	}

	// Handle Struct field access
	if s, ok := t.(*types.Struct); ok {
		for _, f := range s.Fields {
			if f.Name == x.Name.Name {
				c.info.Types[x] = f.Type
				return f.Type
			}
		}
		c.add(diagAt("DTE0001", x.Name.Span, "undefined field '"+x.Name.Name+"' on struct '"+s.Name+"'"))
		return nil
	}

	c.add(diagAt("DTE0005", x.Span, "field access not supported on this type"))
	return nil
}

func (c *checker) resolveDictMethod(x *ast.FieldExpr, d *types.Dict) types.T {
	name := x.Name.Name
	var methodType types.T

	switch name {
	case "get":
		// get(key: K, default: V) -> V
		// TODO: Make default optional?
		methodType = types.FuncOf([]types.T{d.Key, d.Val}, d.Val, false)
	case "has_key":
		// has_key(key: K) -> bool
		methodType = types.FuncOf([]types.T{d.Key}, types.Bool, false)
	case "pop":
		// pop(key: K) -> V
		methodType = types.FuncOf([]types.T{d.Key}, d.Val, false)
	case "clear":
		// clear() -> none
		methodType = types.FuncOf(nil, types.None, false)
	case "keys":
		// keys() -> list[K]
		methodType = types.FuncOf(nil, types.ListOf(d.Key), false)
	case "values":
		// values() -> list[V]
		methodType = types.FuncOf(nil, types.ListOf(d.Val), false)
	case "free":
		// free() -> none (manual memory management)
		methodType = types.FuncOf(nil, types.None, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on dict"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}

func (c *checker) resolveSetMethod(x *ast.FieldExpr, s *types.Set) types.T {
	name := x.Name.Name
	var methodType types.T

	switch name {
	case "add":
		// add(elem: T) -> none
		methodType = types.FuncOf([]types.T{s.Elem}, types.None, false)
	case "remove":
		// remove(elem: T) -> none
		methodType = types.FuncOf([]types.T{s.Elem}, types.None, false)
	case "contains":
		// contains(elem: T) -> bool
		methodType = types.FuncOf([]types.T{s.Elem}, types.Bool, false)
	case "clear":
		// clear() -> none
		methodType = types.FuncOf(nil, types.None, false)
	case "free":
		// free() -> none
		methodType = types.FuncOf(nil, types.None, false)
	case "union":
		// union(other: set[T]) -> set[T]
		methodType = types.FuncOf([]types.T{s}, s, false)
	case "intersection":
		// intersection(other: set[T]) -> set[T]
		methodType = types.FuncOf([]types.T{s}, s, false)
	case "difference":
		// difference(other: set[T]) -> set[T]
		methodType = types.FuncOf([]types.T{s}, s, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on set"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}
