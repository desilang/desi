package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// typFieldExpr handles obj.field or obj.method.
// Currently supports:
// - dict methods: get, has_key, pop, clear, keys, values
// - Class static methods: ClassName.static_method()
func (c *checker) typFieldExpr(x *ast.FieldExpr) types.T {
	// Special case: ClassName.StaticMethod (accessing static method on class type)
	// x.X might be an Ident referring to a class type
	if id, ok := x.X.(*ast.Ident); ok {
		sym := c.scope.Lookup(id.Name)
		if sym != nil && sym.Kind == SymType {
			// Check if it's a Class type with static methods
			if classType, ok := sym.Type.(*types.Class); ok {
				methodName := x.Name.Name
				// Look up static method first
				if staticMethod, exists := classType.StaticMethods[methodName]; exists {
					// Return the static method type (no self parameter)
					c.info.Types[x] = staticMethod
					return staticMethod
				}
				// Look up class method (ClassName.classmethod())
				if classMethod, exists := classType.ClassMethods[methodName]; exists {
					// Return the class method type (cls parameter, but call syntax is ClassName.method())
					c.info.Types[x] = classMethod
					return classMethod
				}
				// If not found in static/class methods, fall through to check constructors/variants
			}

			// Check for enum variant constructor (existing logic)
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

	// Handle Generic types (e.g. Option[int].Some or Box[int].val)
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
						variantParams = append(variantParams, substitute(f.Type, subst))
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
		} else if structT, ok := gen.Base.(*types.Struct); ok {
			// Handle field access on generic struct
			fieldName := x.Name.Name
			for _, f := range structT.Fields {
				if f.Name == fieldName {
					// Map TypeParam name -> Generic Arg
					subst := make(map[string]types.T)
					if len(structT.TypeParams) == len(gen.Args) {
						for i, tp := range structT.TypeParams {
							subst[tp.Name] = gen.Args[i]
						}
					}

					// Substitute type parameters in field type
					fieldType := substitute(f.Type, subst)
					c.info.Types[x] = fieldType
					return fieldType
				}
			}
			c.add(diagAt("DTE0001", x.Name.Span, "undefined field '"+fieldName+"' on generic struct '"+structT.Name+"'"))
			return nil
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

	// Handle List methods
	if l, ok := t.(*types.List); ok {
		return c.resolveListMethod(x, l)
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

	// Handle Class method/field access
	if cls, ok := t.(*types.Class); ok {
		name := x.Name.Name

		// 1. Check fields (including inherited)
		curr := cls
		for curr != nil {
			for _, f := range curr.Fields {
				if f.Name == name {
					// Visibility check: if not pub, must be in same module/file?
					// For now, Desi classes are file-private by default unless pub.
					// But wait, fields are marked IsPub.
					// If !IsPub, access is only allowed from within the class methods?
					// Or same file?
					// Rust: private fields are private to the module.
					// Let's assume same-file for now (simplest).
					// But we don't track file of definition easily here?
					// We can check if we are inside a method of the same class?

					// For MVP: If !IsPub, forbid access unless we are in the same package/file.
					// Actually, let's just enforce: if !IsPub, forbid access from outside.
					// But "outside" needs definition.
					// Let's assume "outside" means "not in a method of this class".

					// Better: check if we are in the same file.
					// c.file is current file. We need definition file of the field.
					// We don't have that easily on types.Field.

					// Alternative: Only enforce if imported from another module?
					// If cls is from another module (how do we know?), then private fields are inaccessible.
					// Types don't track their module.

					// Let's implement a simple rule:
					// If !IsPub, and we are not inside a method of 'cls' (or subclass), error.

					if !f.IsPub {
						// Check if we are inside a method of cls
						// c.curFuncDecl is the current function.
						// We need to know if c.curFuncDecl is a method of cls.
						// We can check if c.curFuncDecl.Recv matches cls.
						// But c.curFuncDecl is AST, cls is Type.

						// Let's skip strict enforcement for same-file for now and just check "IsPub"
						// If it's NOT pub, we warn/error?
						// No, private fields are useful.

						// Let's just enforce: if !IsPub, it is PRIVATE.
						// Access allowed only if we are inside the class definition (methods).
						// How to check that?
						// We can check if `c.scope` contains `self` of type `cls`?

						allowed := false
						if selfSym := c.scope.Lookup("self"); selfSym != nil {
							if selfType, ok := selfSym.Type.(*types.Class); ok {
								if types.Equal(selfType, cls) {
									allowed = true
								}
								// Also allow if subclass?
								// Protected access?
								// Let's stick to private = class-only.
							}
						}

						if !allowed {
							c.add(diagAt("DTE0010", x.Name.Span, "field '"+name+"' is private"))
						}
					}
					return f.Type
				}
			}
			curr = curr.Base
		}

		// ═══════════════════════════════════════════════════════════════════════
		// PROPERTY ACCESS POLICY (DO NOT REMOVE - Design Documentation)
		// ═══════════════════════════════════════════════════════════════════════
		//
		// WHAT: Properties are accessed like fields but call getter functions
		//
		// SYNTAX:
		//   Definition: @property pub def area(self) -> float: ...
		//   Access:     let a = circle.area  (NO parentheses!)
		//
		// HOW IT WORKS:
		//   1. Property stored in cls.Properties map
		//   2. When accessed as instance.prop, return the property's return type
		//   3. Lowering: Becomes a function call at codegen
		//
		// PERFORMANCE:
		//   - Cost: One function call per access
		//   - Mitigation: LLVM can inline trivial properties
		//   - Hot loops: Cache the value: let r = circle.radius; for ...
		//
		// WHY THIS DESIGN:
		//   1. Computed properties are common (area from radius)
		//   2. Encapsulation: Can change field → property without breaking API
		//   3. Acceptable tradeoff: Utility > small perf cost
		//
		// FUTURE OPTIMIZATIONS:
		//   - Inline hint: #[inline] @property for always-inline
		//   - Memoization: @cached_property for expensive computations
		//   - Compile-time eval: @const_property for pure functions
		//
		// HOW TO EXTEND:
		//   - Setters: @property.setter def area(self, val) -> none
		//   - Deleters: @property.deleter def area(self) -> none (rare)
		//
		// ═══════════════════════════════════════════════════════════════════════

		// 1.5. Check properties (instance.property - accessed like field, not method call)
		curr = cls
		for curr != nil {
			if propFunc, exists := curr.Properties[name]; exists {
				// Visibility check
				if !propFunc.IsPub {
					allowed := false
					if selfSym := c.scope.Lookup("self"); selfSym != nil {
						if selfType, ok := selfSym.Type.(*types.Class); ok {
							if types.Equal(selfType, cls) {
								allowed = true
							}
						}
					}
					if !allowed {
						c.add(diagAt("DTE0010", x.Name.Span, "property '"+name+"' is private"))
					}
				}

				// Properties are called automatically, return their return type
				// The property function signature is (self) -> T
				// When accessed as instance.prop, we return T (the return type)
				if propFunc.Ret != nil {
					c.info.Types[x] = propFunc.Ret
					return propFunc.Ret
				}
				// Fallback if no return type
				c.info.Types[x] = types.Any
				return types.Any
			}
			curr = curr.Base
		}

		// 2. Check methods/dunders (including inherited)
		curr = cls
		for curr != nil {
			var method *types.Func
			if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
				method = curr.Dunders[name]
			} else {
				method = curr.Methods[name]
			}

			if method != nil {
				// Visibility check for methods
				if !method.IsPub {
					allowed := false
					if selfSym := c.scope.Lookup("self"); selfSym != nil {
						if selfType, ok := selfSym.Type.(*types.Class); ok {
							// Check if self is of the same class (or subclass) as curr
							// For now: strict check - must be same class
							if types.Equal(selfType, curr) {
								allowed = true
							}
						}
					}
					if !allowed {
						c.add(diagAt("DTE0010", x.Name.Span, "method '"+name+"' is private"))
					}
				}
				// Create bound method type (strip self)
				// Instance methods have self as first param.
				// Exception: __new__ is static, but we are accessing on instance?
				// Accessing __new__ on instance is weird but if we allow it, it has no self.
				// Regular methods have self.

				if name == "__new__" {
					// __new__ has no self, return as is
					c.info.Types[x] = method
					return method
				}

				if len(method.Params) > 0 {
					newParams := method.Params[1:]
					boundMethod := types.FuncOf(newParams, method.Ret, method.Variadic)
					c.info.Types[x] = boundMethod
					return boundMethod
				}
				// Should not happen for instance methods if self injection works
				c.info.Types[x] = method
				return method
			}
			curr = curr.Base
		}

		c.add(diagAt("DTE0001", x.Name.Span, "undefined field or method '"+name+"' on class '"+cls.Name+"'"))
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

func (c *checker) resolveListMethod(x *ast.FieldExpr, l *types.List) types.T {
	name := x.Name.Name
	var methodType types.T

	switch name {
	case "append":
		// append(elem: T) -> void
		methodType = types.FuncOf([]types.T{l.Elem}, types.None, false)
	case "get":
		// get(index: int) -> T
		methodType = types.FuncOf([]types.T{types.Int}, l.Elem, false)
	case "set":
		// set(index: int, elem: T) -> void
		methodType = types.FuncOf([]types.T{types.Int, l.Elem}, types.None, false)
	case "len":
		// len() -> int
		methodType = types.FuncOf(nil, types.Int, false)
	case "pop":
		// pop() -> T
		methodType = types.FuncOf(nil, l.Elem, false)
	case "free":
		// free() -> void
		methodType = types.FuncOf(nil, types.None, false)
	case "insert":
		// insert(index: int, elem: T) -> void
		methodType = types.FuncOf([]types.T{types.Int, l.Elem}, types.None, false)
	case "remove":
		// remove(elem: T) -> void
		methodType = types.FuncOf([]types.T{l.Elem}, types.None, false)
	case "reverse":
		// reverse() -> void
		methodType = types.FuncOf(nil, types.None, false)
	case "clear":
		// clear() -> void
		methodType = types.FuncOf(nil, types.None, false)
	case "copy":
		// copy() -> list[T]
		methodType = types.FuncOf(nil, l, false)
	case "extend":
		// extend(other: list[T]) -> void
		methodType = types.FuncOf([]types.T{l}, types.None, false)
	case "index":
		// index(elem: T) -> int
		methodType = types.FuncOf([]types.T{l.Elem}, types.Int, false)
	case "count":
		// count(elem: T) -> int
		methodType = types.FuncOf([]types.T{l.Elem}, types.Int, false)
	case "contains":
		// contains(elem: T) -> bool
		methodType = types.FuncOf([]types.T{l.Elem}, types.Bool, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on list"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}
