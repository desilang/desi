package check

import (
	"strconv"
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

	// Unwrap TypeAlias to access underlying type for field access
	if alias, ok := t.(*types.TypeAlias); ok {
		t = alias.Target
	}

	// Handle tuple index access: t.0, t.1, etc. or named field access: t.x, t.y
	if tupType, ok := t.(*types.Tuple); ok {
		fieldName := x.Name.Name

		// First, check for named field access (named tuples)
		if tupType.IsNamed() {
			idx := tupType.FieldIndex(fieldName)
			if idx >= 0 {
				res := tupType.Elems[idx]
				c.info.Types[x] = res
				return res
			}
			// Field name not found in named tuple - could still be integer index
		}

		// Check if field name is a valid integer index
		idx, err := strconv.Atoi(fieldName)
		if err != nil {
			if tupType.IsNamed() {
				c.add(diagAt("DTE0006", x.Name.Span, "named tuple has no field '"+fieldName+"'"))
			} else {
				c.add(diagAt("DTE0006", x.Name.Span, "tuple field must be an integer index"))
			}
			return nil
		}
		if idx < 0 || idx >= len(tupType.Elems) {
			c.add(diagAt("DTE0007", x.Name.Span, "tuple index out of bounds"))
			return nil
		}
		res := tupType.Elems[idx]
		c.info.Types[x] = res
		return res
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
		} else if classT, ok := gen.Base.(*types.Class); ok {
			// Handle field/method access on generic class (e.g., Box[int].val, Box[int].get())
			name := x.Name.Name

			// Build substitution map: TypeParam name -> Generic Arg
			subst := make(map[string]types.T)
			if len(classT.TypeParams) == len(gen.Args) {
				for i, tp := range classT.TypeParams {
					subst[tp.Name] = gen.Args[i]
				}
			}

			// 1. Check fields (including inherited)
			curr := classT
			for curr != nil {
				for _, f := range curr.Fields {
					if f.Name == name {
						// Apply substitution to field type
						fieldType := substitute(f.Type, subst)
						c.info.Types[x] = fieldType
						return fieldType
					}
				}
				curr = curr.Base
			}

			// 2. Check properties
			curr = classT
			for curr != nil {
				if propFunc, exists := curr.Properties[name]; exists {
					// Apply substitution to property return type
					retType := substitute(propFunc.Ret, subst)
					c.info.Types[x] = retType
					return retType
				}
				curr = curr.Base
			}

			// 3. Check methods/dunders (including inherited)
			curr = classT
			for curr != nil {
				var method *types.Func
				if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
					method = curr.Dunders[name]
				} else {
					method = curr.Methods[name]
				}

				if method != nil {
					// Apply substitution to method parameters and return type
					newParams := make([]types.T, len(method.Params))
					for i, p := range method.Params {
						newParams[i] = substitute(p, subst)
					}
					newRet := substitute(method.Ret, subst)

					// Create bound method type (strip self for instance access)
					if len(newParams) > 0 {
						boundParams := newParams[1:] // Skip self
						boundMethod := types.FuncOf(boundParams, newRet, method.Variadic)
						c.info.Types[x] = boundMethod
						return boundMethod
					}
					// Should not happen for instance methods
					boundMethod := types.FuncOf(newParams, newRet, method.Variadic)
					c.info.Types[x] = boundMethod
					return boundMethod
				}
				curr = curr.Base
			}

			c.add(diagAt("DTE0001", x.Name.Span, "undefined field or method '"+name+"' on generic class '"+classT.Name+"'"))
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

	// Handle str methods: split, replace
	if types.Equal(t, types.Str) {
		return c.resolveStrMethod(x)
	}

	// Handle File methods: read, write, close, is_open
	if types.Equal(t, types.File) {
		return c.resolveFileMethod(x)
	}

	// Handle Arena methods: alloc
	if _, ok := t.(*types.Arena); ok {
		return c.resolveArenaMethod(x)
	}

	// Handle Option methods
	if types.IsOption(t) {
		return c.checkOptionMethod(x, t)
	}

	// Handle Result methods
	if types.IsResult(t) {
		return c.checkResultMethod(x, t)
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
					c.info.Types[x] = f.Type
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
		// Determine if this is a static access (via ClassName) or instance access
		isTypeAccess := false
		if id, ok := x.X.(*ast.Ident); ok {
			sym := c.scope.Lookup(id.Name)
			if sym != nil && sym.Kind == SymType {
				isTypeAccess = true
			}
		}

		// 2.0 Check Class Constants (only if accessing via ClassName)
		if isTypeAccess {
			curr = cls
			for curr != nil {
				if cnst, ok := curr.Constants[name]; ok {
					// Check visibility
					if !cnst.IsPub {
						allowed := false
						// Allow if we are inside the class (or subclass)
						if selfSym := c.scope.Lookup("self"); selfSym != nil {
							if selfType, ok := selfSym.Type.(*types.Class); ok {
								// Check if selfType is subclass of cls (where constant is defined)
								// Note: cls is the class we are accessing (e.g. Math).
								// If we are in Circle (subclass of Math), self is Circle.
								// Circle is subclass of Math. So allowed.
								// If we are in Math, self is Math. Allowed.
								if types.IsSubclass(selfType, cls) {
									allowed = true
								}
							}
						}

						if !allowed {
							c.add(diagAt("DTE0010", x.Name.Span, "constant '"+name+"' is private"))
						}
					}

					c.info.Types[x] = cnst.Type
					return cnst.Type
				}
				curr = curr.Base
			}

			// 2.1 Check Static Fields (only if accessing via ClassName)
			curr = cls
			for curr != nil {
				if sf, ok := curr.StaticFields[name]; ok {
					// Check visibility
					if !sf.IsPub {
						allowed := false
						// Allow if we are inside the class (or subclass)
						if selfSym := c.scope.Lookup("self"); selfSym != nil {
							if selfType, ok := selfSym.Type.(*types.Class); ok {
								if types.IsSubclass(selfType, cls) {
									allowed = true
								}
							}
						}

						if !allowed {
							c.add(diagAt("DTE0010", x.Name.Span, "static field '"+name+"' is private"))
						}
					}

					c.info.Types[x] = sf.Type
					return sf.Type
				}
				curr = curr.Base
			}
		}

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
					// Allow if same module (file-private)
					// Note: types.Func doesn't store module, so we assume same-module if we can see it?
					// But we can't verify module.
					// Fallback: Allow if inside a method of the same class OR subclass.

					if selfSym := c.scope.Lookup("self"); selfSym != nil {
						if selfType, ok := selfSym.Type.(*types.Class); ok {
							// Check if self is of the same class (or subclass) as curr
							if types.IsSubclass(selfType, curr) {
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

				if name == "__new__" {
					// __new__ has no self, return as is
					c.info.Types[x] = method
					return method
				}

				// If accessed via ClassName.method, return UNBOUND method (keep self)
				if isTypeAccess {
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
	case "insert":
		// insert(key: K, value: V) -> none
		methodType = types.FuncOf([]types.T{d.Key, d.Val}, types.None, false)
	case "has_key":
		// has_key(key: K) -> bool
		methodType = types.FuncOf([]types.T{d.Key}, types.Bool, false)
	case "pop":
		// pop(key: K) -> V
		methodType = types.FuncOf([]types.T{d.Key}, d.Val, false)
	case "clear":
		// clear() -> none (MUTATION)
		c.checkMutableCollection(x, "clear")
		methodType = types.FuncOf(nil, types.None, false)
	case "keys":
		// keys() -> list[K]
		methodType = types.FuncOf(nil, types.ListOf(d.Key), false)
	case "values":
		// values() -> list[V]
		methodType = types.FuncOf(nil, types.ListOf(d.Val), false)
	case "items":
		// items() -> iterable of (K, V) pairs
		// For dict iteration lowering, we return a marker type
		// The lowering handles this specially for `for k, v in dict.items():`
		methodType = types.FuncOf(nil, d, false) // Returns the dict itself as marker
	case "free":
		// free() -> none (manual memory management)
		methodType = types.FuncOf(nil, types.None, false)
	case "__len__":
		// __len__() -> int
		methodType = types.FuncOf(nil, types.Int, false)
	case "to_str":
		// to_str() -> str
		methodType = types.FuncOf(nil, types.Str, false)
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
		// clear() -> none (MUTATION)
		c.checkMutableCollection(x, "clear")
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
	case "__len__":
		// __len__() -> int
		methodType = types.FuncOf(nil, types.Int, false)
	case "to_str":
		// to_str() -> str
		methodType = types.FuncOf(nil, types.Str, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on set"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}

// checkMutableCollection verifies that a mutation method is being called on a mutable collection.
// Returns true if the check passes, false if an error was emitted.
func (c *checker) checkMutableCollection(fe *ast.FieldExpr, methodName string) bool {
	// Get the receiver expression
	// If it's an identifier, check if it's mutable
	if id, ok := fe.X.(*ast.Ident); ok {
		sym := c.scope.Lookup(id.Name)
		if sym != nil && !sym.IsMutable {
			c.add(diagAt("DTE0004", fe.Name.Span,
				"cannot call '"+methodName+"' on immutable collection '"+id.Name+"' (use 'let mut' instead of 'let')"))
			return false
		}
	}
	return true
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
	case "__len__":
		// __len__() -> int
		methodType = types.FuncOf(nil, types.Int, false)
	case "to_str":
		// to_str() -> str
		methodType = types.FuncOf(nil, types.Str, false)
	case "join":
		// join(delim: str) -> str (only for list<str>)
		if l.Elem == types.Str {
			methodType = types.FuncOf([]types.T{types.Str}, types.Str, false)
		} else {
			c.add(diagAt("DTE0001", x.Name.Span, "join() is only available on list<str>"))
			return nil
		}
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on list"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}

func (c *checker) checkOptionMethod(x *ast.FieldExpr, t types.T) types.T {
	name := x.Name.Name
	var methodType types.T

	// Get T from Option<T>
	elemType := types.OptionSomeType(t)
	if elemType == nil {
		// Should not happen if IsOption(t) is true
		elemType = types.Any // Fallback
	}

	switch name {
	case "is_some", "is_none", "is_nothing":
		// () -> bool
		methodType = types.FuncOf(nil, types.Bool, false)
	case "unwrap":
		// () -> T
		methodType = types.FuncOf(nil, elemType, false)
	case "unwrap_or":
		// (default: T) -> T
		methodType = types.FuncOf([]types.T{elemType}, elemType, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on Option"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}

func (c *checker) checkResultMethod(x *ast.FieldExpr, t types.T) types.T {
	name := x.Name.Name
	var methodType types.T

	// Get T and E from Result<T, E>
	okType := types.ResultOkType(t)
	errType := types.ResultErrType(t)

	if okType == nil {
		okType = types.Any
	}
	if errType == nil {
		errType = types.Any
	}

	switch name {
	case "is_ok", "is_err":
		// () -> bool
		methodType = types.FuncOf(nil, types.Bool, false)
	case "unwrap":
		// () -> T
		methodType = types.FuncOf(nil, okType, false)
	case "unwrap_err":
		// () -> E
		methodType = types.FuncOf(nil, errType, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on Result"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}

// resolveStrMethod handles str.split() and str.replace() methods
func (c *checker) resolveStrMethod(x *ast.FieldExpr) types.T {
	name := x.Name.Name
	var methodType types.T

	switch name {
	case "split":
		// split(delim: str) -> list<str>
		methodType = types.FuncOf([]types.T{types.Str}, types.ListOf(types.Str), false)
	case "replace":
		// replace(old: str, new: str) -> str
		methodType = types.FuncOf([]types.T{types.Str, types.Str}, types.Str, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on str"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}

// resolveFileMethod handles File.read(), File.write(), File.close(), File.is_open()
func (c *checker) resolveFileMethod(x *ast.FieldExpr) types.T {
	name := x.Name.Name
	var methodType types.T

	switch name {
	case "read":
		// read() -> str
		methodType = types.FuncOf(nil, types.Str, false)
	case "write":
		// write(data: str) -> none
		methodType = types.FuncOf([]types.T{types.Str}, types.None, false)
	case "close":
		// close() -> none
		methodType = types.FuncOf(nil, types.None, false)
	case "is_open":
		// is_open() -> bool
		methodType = types.FuncOf(nil, types.Bool, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on File"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}

// resolveArenaMethod handles arena.alloc()
func (c *checker) resolveArenaMethod(x *ast.FieldExpr) types.T {
	name := x.Name.Name
	var methodType types.T

	switch name {
	case "alloc":
		// alloc(size: int) -> ptr (opaque pointer to arena-allocated memory)
		// For type checking, we return Any since the caller decides the type
		methodType = types.FuncOf([]types.T{types.Int}, types.Any, false)
	default:
		c.add(diagAt("DTE0001", x.Name.Span, "undefined method '"+name+"' on arena"))
		return nil
	}

	c.info.Types[x] = methodType
	return methodType
}
