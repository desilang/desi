package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// LowerClassConstructor generates the constructor function for a class
// POLICY: If no __new__, generate zero-arg constructor with default initialization
// POLICY: If __new__ exists, lower the user-defined __new__ method
// LowerClassConstructor generates the constructor function for a class
// POLICY: If no __new__, generate zero-arg constructor with default initialization
// POLICY: If __new__ exists, lower the user-defined __new__ method
func LowerClassConstructor(cd *ast.ClassDecl, info *check.Info, src []byte, globals map[string]bool) []*hir.Func {
	return LowerClassConstructorWithName(cd, info, src, cd.Name.Name, globals)
}

// LowerClassConstructorWithName is like LowerClassConstructor but uses an explicit name.
// This is needed for nested classes where the name should be Parent_Child instead of just Child.
func LowerClassConstructorWithName(cd *ast.ClassDecl, info *check.Info, src []byte, className string, globals map[string]bool) []*hir.Func {
	var cls *types.Class
	if t := info.Types[cd]; t != nil {
		cls, _ = t.(*types.Class)
	}

	// Check if __new__ is defined
	hasNew := false
	var newMethod *ast.FuncDecl
	for _, method := range cd.Methods {
		if method.Name.Name == "__new__" {
			hasNew = true
			newMethod = method
			break
		}
	}

	if hasNew {
		// Lower user-defined __new__
		return LowerDunderNew(className, newMethod, info, src, cls, globals)
	} else {
		// Generate default zero-arg constructor (returns slice of [wrapper, __new__])
		return LowerDefaultConstructor(className, cls)
	}
}

// LowerDefaultConstructor generates a zero-arg constructor
// Returns TWO functions:
// 1. ClassName___new__(self) - the initializer
// 2. ClassName() -> ptr - the allocator wrapper that mallocs + calls __new__
func LowerDefaultConstructor(className string, cls *types.Class) []*hir.Func {
	// 1. Generate the __new__ method
	ctorName := fmt.Sprintf("%s___new__", className)
	newFn := hir.NewFunc(ctorName)
	newFn.Func().Params = []hir.Param{{Name: "self", Type: "ptr"}}
	newFn.Func().RetType = "void"

	entry := hir.NewBlock("entry")
	entry.Stmts = append(entry.Stmts, &hir.Ret{})
	newFn.Func().Blocks = []*hir.Block{entry}

	// 2. Generate the allocator wrapper
	wrapper := hir.NewFunc(className)
	wrapper.Func().Params = nil // zero-arg
	wrapper.Func().RetType = "ptr"

	wrapperEntry := hir.NewBlock("entry")

	// Calculate size
	totalSize := 0
	if cls != nil {
		for _, field := range cls.Fields {
			totalSize += getSize(field.Type)
		}
	}
	if totalSize == 0 {
		totalSize = 1
	}

	// Allocate
	instancePtr := hir.Temp{Name: "%instance"}
	wrapperEntry.Stmts = append(wrapperEntry.Stmts, &hir.Call{
		Dst:  instancePtr,
		Fn:   "malloc",
		Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", totalSize)}},
		Type: "ptr",
	})

	// Call __new__
	wrapperEntry.Stmts = append(wrapperEntry.Stmts, &hir.Call{
		Fn:   ctorName,
		Args: []hir.Value{instancePtr},
	})

	// Return instance
	wrapperEntry.Stmts = append(wrapperEntry.Stmts, &hir.Ret{Val: instancePtr})

	wrapper.Func().Blocks = []*hir.Block{wrapperEntry}

	return []*hir.Func{wrapper.Func(), newFn.Func()}
}

// LowerDunderNew lowers a user-defined __new__ method
func LowerDunderNew(className string, method *ast.FuncDecl, info *check.Info, src []byte, cls *types.Class, globals map[string]bool) []*hir.Func {
	// 1. Lower the user's __new__ method as ClassName___new__
	// This method takes (self, args...)
	// Pass className context so return ClassName(field=val) initializes self instead of allocating
	selfPtr := hir.Temp{Name: "%self"}
	newFn := LowerFuncForDunderNew(method, info, src, className, selfPtr, globals)
	newFn.Name = fmt.Sprintf("%s___new__", className)

	// Ensure __new__ returns void (it initializes self, doesn't return a new instance)
	newFn.RetType = "void"

	// Ensure first param is self: ptr
	if len(newFn.Params) == 0 || newFn.Params[0].Name != "self" {
		newFn.Params = append([]hir.Param{{Name: "self", Type: "ptr"}}, newFn.Params...)
	}

	// 2. Generate the constructor wrapper: ClassName(args...) -> ptr
	// This wrapper allocates memory, calls __new__, and returns the instance
	wrapper := hir.NewFunc(className)

	// Copy params from __new__ but skip the first one (self)
	if len(newFn.Params) > 1 {
		wrapper.Func().Params = make([]hir.Param, len(newFn.Params)-1)
		for i := 1; i < len(newFn.Params); i++ {
			wrapper.Func().Params[i-1] = newFn.Params[i]
		}
	}
	wrapper.Func().RetType = "ptr"

	entry := hir.NewBlock("entry")

	// Calculate total size
	totalSize := 0
	if cls != nil {
		for _, field := range cls.Fields {
			totalSize += getSize(field.Type)
		}
	}
	if totalSize == 0 {
		totalSize = 1
	}

	// Allocate
	instancePtr := hir.Temp{Name: "%instance"}
	entry.Stmts = append(entry.Stmts, &hir.Call{
		Dst:  instancePtr,
		Fn:   "malloc",
		Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", totalSize)}},
		Type: "ptr",
	})

	// Call __new__(instance, args...)
	callArgs := make([]hir.Value, 0, len(wrapper.Func().Params)+1)
	callArgs = append(callArgs, instancePtr)
	for _, p := range wrapper.Func().Params {
		callArgs = append(callArgs, hir.Temp{Name: "%" + p.Name})
	}

	entry.Stmts = append(entry.Stmts, &hir.Call{
		Fn:   newFn.Name,
		Args: callArgs,
	})

	// Return instance
	entry.Stmts = append(entry.Stmts, &hir.Ret{Val: instancePtr})

	wrapper.Func().Blocks = []*hir.Block{entry}

	return []*hir.Func{wrapper.Func(), newFn}
}

// LowerClassMethods generates HIR functions for all class methods
// POLICY: Methods have implicit self, so we inject it as first parameter in HIR
func LowerClassMethods(cd *ast.ClassDecl, info *check.Info, src []byte, globals map[string]bool) []*hir.Func {
	return LowerClassMethodsWithName(cd, info, src, cd.Name.Name, globals)
}

// LowerClassMethodsWithName is like LowerClassMethods but uses an explicit name.
// This is needed for nested classes where the name should be Parent_Child.
func LowerClassMethodsWithName(cd *ast.ClassDecl, info *check.Info, src []byte, className string, globals map[string]bool) []*hir.Func {
	var funcs []*hir.Func

	for _, method := range cd.Methods {
		methodName := method.Name.Name

		// Skip __new__ (handled by constructor)
		if methodName == "__new__" {
			continue
		}

		// Lower method as function
		fn := LowerFuncFromDecl(method, info, src, globals)

		// POLICY: Static dispatch with name mangling
		fn.Name = fmt.Sprintf("%s_%s", className, methodName)

		// ═══════════════════════════════════════════════════════════════════════
		// LOWERING DECORATOR POLICY (DO NOT REMOVE - Design Documentation)
		// ═══════════════════════════════════════════════════════════════════════
		//
		// SELF/CLS PARAMETER INJECTION:
		//
		// Instance methods:  def method(self, x) -> T
		//                   Lowered: fn(self: ptr, x: T) -> T
		//
		// @staticmethod:    def method(x) -> T
		//                   Lowered: fn(x: T) -> T  (NO self!)
		//
		// @classmethod:     def method(x) -> T
		//                   Lowered: fn(x: T) -> T  (NO cls!)
		//                   Note: Use class name in body, not cls param
		//
		// @property:        def prop(self) -> T
		//                   Lowered: fn(self: ptr) -> T  (regular instance method)
		//                   Called automatically when accessed as instance.prop
		//
		// FUTURE EXTENSIONS:
		//   - When adding class variables: @classmethod may need cls injection
		//     Format: fn(cls: Type[ClassName], args...) -> T
		//   - Property setters: def prop(self, val) -> none
		//     Lowered: fn(self: ptr, val: T) -> none
		//
		// ═══════════════════════════════════════════════════════════════════════

		// Check decorators
		isStatic := false
		isClassMethod := false
		for _, dec := range method.Decorators {
			if dec != nil {
				if dec.Name.Name == "staticmethod" {
					isStatic = true
				} else if dec.Name.Name == "classmethod" {
					isClassMethod = true
				}
			}
		}

		// Only inject self for regular instance methods
		// Static methods and class methods: NO parameter injection
		if !isStatic && !isClassMethod {
			// Check if first param is already self (explicit)
			hasSelf := false
			if len(fn.Params) > 0 && fn.Params[0].Name == "self" {
				hasSelf = true
			}
			if !hasSelf {
				fn.Params = append([]hir.Param{{Name: "self", Type: "ptr"}}, fn.Params...)
			}
		}

		funcs = append(funcs, fn)
	}

	// -------------------------------------------------------------------------
	// LOWERING INHERITED METHODS (Monomorphization Strategy)
	// -------------------------------------------------------------------------
	// We must emit code for methods inherited from base classes, especially if
	// the base class is generic and we are a concrete instantiation (or distinct class).
	// Iterate through ALL methods known to the type system (inherited included).

	var cls *types.Class
	if t := info.Types[cd]; t != nil {
		cls, _ = t.(*types.Class)
	}

	if cls != nil {
		// Use a set to track methods we already emitted from the AST
		emitted := make(map[string]bool)
		for _, m := range cd.Methods {
			emitted[m.Name.Name] = true
		}

		// Build substitution map if inheriting from a generic class instantiation
		// e.g., IntBox(Box<int>) -> subst["T"] = int
		// We use the AST base type args (cd.Bases[0].Params) and match them
		// to the base class type params (cls.Base.TypeParams)
		var subst map[string]types.T
		if len(cd.Bases) > 0 && cls.Base != nil && len(cls.Base.TypeParams) > 0 {
			baseTypeName := cd.Bases[0]
			if len(baseTypeName.Params) == len(cls.Base.TypeParams) {
				subst = make(map[string]types.T)
				for i, tp := range cls.Base.TypeParams {
					// Resolve the AST type name to a types.T
					argType := resolveASTTypeNameToT(baseTypeName.Params[i], info)
					if argType != nil {
						subst[tp.Name] = argType
					}
				}
			}
		}

		// Iterate over all semantic methods (including inherited)
		for name := range cls.Methods {
			// Skip if already emitted (overridden or defined locally)
			if emitted[name] {
				continue
			}

			// Skip __new__ (handled by constructor logic separately)
			if name == "__new__" {
				continue
			}

			// Find the original AST declaration by walking up the inheritance chain
			astDecl := findMethodDecl(cls.Base, name)
			if astDecl == nil {
				// Should not happen if type checker is correct, but safer to skip
				continue
			}

			// Lower the inherited method as if it belongs to this class
			fn := LowerFuncFromDecl(astDecl, info, src, globals)

			// Apply type substitution if inheriting from a generic class
			// This replaces T with concrete types (e.g., int) in method body
			if subst != nil && cls.Base != nil {
				substituteHIRFuncBody(fn, subst, cls.Base)
			}

			// Update return type and parameter types using the already-substituted
			// method type from cls.Methods (type checker already applied substitution)
			if methodType := cls.Methods[name]; methodType != nil {
				// Update return type
				if methodType.Ret != nil {
					fn.RetType = lowerType(methodType.Ret)
				}
			}

			// Relabel it for THIS class: IntBox_get
			fn.Name = fmt.Sprintf("%s_%s", className, name)

			// Logic for self injection must match the original AST metadata (static, etc)
			// LowerFuncFromDecl handles the body. We need to apply parameter policy (Self injection).
			// We replicate the policy logic here:

			isStatic := false
			isClassMethod := false
			for _, dec := range astDecl.Decorators {
				if dec != nil {
					if dec.Name.Name == "staticmethod" {
						isStatic = true
					} else if dec.Name.Name == "classmethod" {
						isClassMethod = true
					}
				}
			}

			if !isStatic && !isClassMethod {
				hasSelf := false
				if len(fn.Params) > 0 && fn.Params[0].Name == "self" {
					hasSelf = true
				}
				if !hasSelf {
					fn.Params = append([]hir.Param{{Name: "self", Type: "ptr"}}, fn.Params...)
				}
			}

			funcs = append(funcs, fn)

		}
	}

	return funcs
}

// findMethodDecl walks up the base chain to find the AST declaration of a method
func findMethodDecl(cls *types.Class, name string) *ast.FuncDecl {
	if cls == nil {
		return nil
	}
	// Check if this class defines it in its AST
	if cls.Decl != nil {
		for _, m := range cls.Decl.Methods {
			if m.Name.Name == name {
				return m
			}
		}
	}
	// Recurse up
	return findMethodDecl(cls.Base, name)
}

// resolveASTTypeNameToT converts an AST TypeName to a types.T for substitution.
// This handles common built-in types and class types.
func resolveASTTypeNameToT(tn *ast.TypeName, info *check.Info) types.T {
	if tn == nil {
		return nil
	}

	// Handle basic types
	switch tn.Name {
	case "int":
		return types.Int
	case "float":
		return types.Float
	case "bool":
		return types.Bool
	case "str":
		return types.Str
	case "none":
		return types.None
	case "any":
		return types.Any
	case "i8":
		return types.I8
	case "i16":
		return types.I16
	case "i32":
		return types.I32
	case "i64":
		return types.I64
	case "u8":
		return types.U8
	case "u16":
		return types.U16
	case "u32":
		return types.U32
	case "u64":
		return types.U64
	case "f32":
		return types.F32
	case "f64":
		return types.F64
	case "list":
		if len(tn.Params) > 0 {
			elem := resolveASTTypeNameToT(tn.Params[0], info)
			return &types.List{Elem: elem}
		}
		return &types.List{Elem: types.Any}
	case "dict":
		if len(tn.Params) >= 2 {
			key := resolveASTTypeNameToT(tn.Params[0], info)
			val := resolveASTTypeNameToT(tn.Params[1], info)
			return &types.Dict{Key: key, Val: val}
		}
		return &types.Dict{Key: types.Any, Val: types.Any}
	case "set":
		if len(tn.Params) > 0 {
			elem := resolveASTTypeNameToT(tn.Params[0], info)
			return &types.Set{Elem: elem}
		}
		return &types.Set{Elem: types.Any}
	}

	// Try to find the type in Info.Types by looking up known class declarations
	// This handles user-defined classes
	for node, t := range info.Types {
		switch decl := node.(type) {
		case *ast.ClassDecl:
			if decl.Name.Name == tn.Name {
				return t
			}
		case *ast.StructDecl:
			if decl.Name.Name == tn.Name {
				return t
			}
		case *ast.EnumDecl:
			if decl.Name.Name == tn.Name {
				return t
			}
		}
	}

	return nil
}
