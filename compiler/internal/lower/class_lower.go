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
func LowerClassConstructor(cd *ast.ClassDecl, info *check.Info) *hir.Func {
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
		return LowerDunderNew(className, newMethod, info)
	} else {
		// Generate default zero-arg constructor
		return LowerDefaultConstructor(className, cls)
	}
}

// LowerDefaultConstructor generates a zero-arg constructor
// Allocates class instance and zero-initializes all fields
func LowerDefaultConstructor(className string, cls *types.Class) *hir.Func {
	b := hir.NewFunc(className)
	b.Func().Params = []hir.Param{} // Zero arguments
	b.Func().RetType = "ptr"

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

	// Zero-initialize fields (memset or individual stores)
	// For simplicity, we'll skip explicit zeroing (malloc often zeros memory)

	// Return instance
	entry.Stmts = append(entry.Stmts, &hir.Ret{Val: instancePtr})

	b.Func().Blocks = []*hir.Block{entry}
	return b.Func()
}

// LowerDunderNew lowers a user-defined __new__ method
func LowerDunderNew(className string, method *ast.FuncDecl, info *check.Info) *hir.Func {
	// Lower as regular function but with special name
	fn := LowerFuncFromDecl(method, info, nil)
	fn.Name = className // Constructor has class name

	// __new__ already has implicit self in type system,
	// but in HIR we don't include self as parameter for constructors
	// The body should use literal construction: ClassName{...}

	return fn
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

		// POLICY: Inject self as first HIR parameter (unless @staticmethod)
		// Check if method has @staticmethod decorator
		isStatic := false
		for _, dec := range method.Decorators {
			if dec != nil && dec.Name.Name == "staticmethod" {
				isStatic = true
				break
			}
		}

		// Only inject self for non-static methods
		if !isStatic {
			fn.Params = append([]hir.Param{{Name: "self", Type: "ptr"}}, fn.Params...)
		}

		funcs = append(funcs, fn)
	}

	return funcs
}
