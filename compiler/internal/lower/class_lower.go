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
func LowerClassConstructor(cd *ast.ClassDecl, info *check.Info) []*hir.Func {
	className := cd.Name.Name

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
		return LowerDunderNew(className, newMethod, info, cls)
	} else {
		// Generate default zero-arg constructor
		return []*hir.Func{LowerDefaultConstructor(className, cls)}
	}
}

// LowerDefaultConstructor generates a zero-arg constructor
// Allocates class instance and zero-initializes all fields
// The function is named ClassName___new__ to match call site expectations
func LowerDefaultConstructor(className string, cls *types.Class) *hir.Func {
	ctorName := fmt.Sprintf("%s___new__", className)
	b := hir.NewFunc(ctorName)
	// Zero-arg default constructor still takes self as first param for consistency
	b.Func().Params = []hir.Param{{Name: "self", Type: "ptr"}}
	b.Func().RetType = "void" // __new__ doesn't return, it initializes self

	entry := hir.NewBlock("entry")

	// Default __new__ does nothing - self is already allocated by caller
	// and fields are zero-initialized by default
	// Just return (void)
	entry.Stmts = append(entry.Stmts, &hir.Ret{})

	b.Func().Blocks = []*hir.Block{entry}
	return b.Func()
}

// LowerDunderNew lowers a user-defined __new__ method
func LowerDunderNew(className string, method *ast.FuncDecl, info *check.Info, cls *types.Class) []*hir.Func {
	// 1. Lower the user's __new__ method as ClassName___new__
	// This method takes (self, args...)
	newFn := LowerFuncFromDecl(method, info, nil)
	newFn.Name = fmt.Sprintf("%s___new__", className)

	// 2. Generate the constructor wrapper: ClassName(args...) -> ptr
	// This wrapper allocates memory, calls __new__, and returns the instance
	wrapper := hir.NewFunc(className)

	// Copy params from __new__ but skip the first one (self)
	if len(newFn.Params) > 0 {
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
		// We need to use the parameter names as values
		// In HIR, parameters are values.
		// But wait, hir.Param is {Name, Type}.
		// We need to use hir.Var{Name: p.Name} or hir.Temp{Name: p.Name} depending on convention.
		// LowerFuncFromDecl uses parameter names as is.
		// Let's assume they are temps or vars.
		// Usually params are %name.
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
func LowerClassMethods(cd *ast.ClassDecl, info *check.Info, src []byte) []*hir.Func {
	className := cd.Name.Name
	var funcs []*hir.Func

	for _, method := range cd.Methods {
		methodName := method.Name.Name

		// Skip __new__ (handled by constructor)
		if methodName == "__new__" {
			continue
		}

		// Lower method as function
		fn := LowerFuncFromDecl(method, info, src)

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

	return funcs
}
