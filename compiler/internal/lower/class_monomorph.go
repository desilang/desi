package lower

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// LowerMonomorphizedClass generates specialized versions of a generic class for each
// concrete instantiation found during type checking.
//
// For example, if Box<T> is instantiated as Box<int> and Box<str>, this will generate:
// - Box_int___new__, Box_int_get, Box_int_set
// - Box_str___new__, Box_str_get, Box_str_set
func LowerMonomorphizedClass(cd *ast.ClassDecl, info *check.Info, src []byte) []*hir.Func {
	// Get base class type for field/method info
	var baseCls *types.Class
	if t := info.Types[cd]; t != nil {
		baseCls, _ = t.(*types.Class)
	}

	if baseCls == nil {
		return nil
	}

	className := baseCls.Name // Use fully qualified name from type (e.g., Container.Box)
	instantiations := info.ClassInstantiations[className]
	if len(instantiations) == 0 {
		return nil
	}

	var funcs []*hir.Func

	// Track processed instantiations to avoid duplicates
	processed := make(map[string]bool)

	// For each instantiation, generate specialized versions
	for _, gen := range instantiations {
		// Generate mangled name to check for duplicates
		mangledName := mangleGenericClassName(className, gen.Args)

		// Skip if already processed
		if processed[mangledName] {
			continue
		}
		processed[mangledName] = true

		// Build substitution map: TypeParam -> ConcreteType
		subst := make(map[string]types.T)
		if baseCls != nil && len(baseCls.TypeParams) == len(gen.Args) {
			for i, tp := range baseCls.TypeParams {
				subst[tp.Name] = gen.Args[i]
			}
		}

		// Generate constructor
		ctors := lowerMonomorphizedConstructor(cd, mangledName, subst, info, baseCls)
		funcs = append(funcs, ctors...)

		// Generate methods
		methods := lowerMonomorphizedMethods(cd, mangledName, subst, info, baseCls, src)
		funcs = append(funcs, methods...)
	}

	return funcs
}

// mangleGenericClassName generates a mangled name for a generic class instantiation.
// Box<int> -> Box_int, Pair<int, str> -> Pair_int_str
// For nested classes like Container.Inner, the dot is replaced with underscore.
func mangleGenericClassName(base string, args []types.T) string {
	// Replace dots with underscores for nested classes (Container.Inner -> Container_Inner)
	safeBase := strings.ReplaceAll(base, ".", "_")

	if len(args) == 0 {
		return safeBase
	}

	var parts []string
	for _, arg := range args {
		parts = append(parts, mangleTypeName(arg))
	}
	return fmt.Sprintf("%s_%s", safeBase, strings.Join(parts, "_"))
}

// mangleTypeName converts a type to a name-safe string for mangling.
func mangleTypeName(t types.T) string {
	if t == nil {
		return "any"
	}

	// Check basic types (singletons)
	if t == types.Int {
		return "int"
	}
	if t == types.Float {
		return "float"
	}
	if t == types.Bool {
		return "bool"
	}
	if t == types.Str {
		return "str"
	}
	if t == types.None {
		return "none"
	}

	switch x := t.(type) {
	case *types.Class:
		return x.Name
	case *types.Struct:
		return x.Name
	case *types.Enum:
		return x.Name
	case *types.List:
		return "list_" + mangleTypeName(x.Elem)
	case *types.Dict:
		return "dict_" + mangleTypeName(x.Key) + "_" + mangleTypeName(x.Val)
	case *types.Set:
		return "set_" + mangleTypeName(x.Elem)
	case *types.Generic:
		if cls, ok := x.Base.(*types.Class); ok {
			return mangleGenericClassName(cls.Name, x.Args)
		}
		return "generic"
	default:
		return strings.ReplaceAll(t.String(), " ", "_")
	}
}

// lowerMonomorphizedConstructor generates specialized constructors.
func lowerMonomorphizedConstructor(cd *ast.ClassDecl, mangledName string, subst map[string]types.T, info *check.Info, baseCls *types.Class) []*hir.Func {
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
		// Lower user-defined __new__ with substitution
		return lowerMonomorphizedDunderNew(mangledName, newMethod, subst, info, baseCls)
	}

	// Generate default zero-arg constructor
	return []*hir.Func{lowerMonomorphizedDefaultConstructor(mangledName, subst, baseCls)}
}

// lowerMonomorphizedDefaultConstructor generates a zero-arg constructor for a specialized class.
func lowerMonomorphizedDefaultConstructor(mangledName string, subst map[string]types.T, baseCls *types.Class) *hir.Func {
	ctorName := fmt.Sprintf("%s___new__", mangledName)
	b := hir.NewFunc(ctorName)
	b.Func().Params = []hir.Param{{Name: "self", Type: "ptr"}}
	b.Func().RetType = "void"

	entry := hir.NewBlock("entry")
	entry.Stmts = append(entry.Stmts, &hir.Ret{})
	b.Func().Blocks = []*hir.Block{entry}
	return b.Func()
}

// lowerMonomorphizedDunderNew generates a specialized __new__ method.
func lowerMonomorphizedDunderNew(mangledName string, method *ast.FuncDecl, subst map[string]types.T, info *check.Info, baseCls *types.Class) []*hir.Func {
	// Lower the __new__ method
	newFn := LowerFuncFromDecl(method, info, nil)
	newFn.Name = fmt.Sprintf("%s___new__", mangledName)

	// Get constructor's semantic type from the class for proper substitution
	var ctorType *types.Func
	if baseCls != nil && len(baseCls.Constructors) > 0 {
		// Find a matching constructor by arity
		for _, ctor := range baseCls.Constructors {
			if len(ctor.Params) == len(newFn.Params) {
				ctorType = ctor
				break
			}
		}
	}

	// Substitute parameter types using semantic type info
	if ctorType != nil && len(ctorType.Params) > 0 {
		// ctorType.Params[0] is self (cls), params after that are user params
		for i := 1; i < len(ctorType.Params) && i < len(newFn.Params); i++ {
			paramType := substituteType(ctorType.Params[i], subst)
			newFn.Params[i].Type = lowerType(paramType)
		}
	}
	newFn.RetType = "void" // __new__ returns void

	// Also substitute types in function body
	substituteHIRFuncBody(newFn, subst, baseCls)

	// Generate wrapper constructor: MangledName(args...) -> ptr
	wrapper := hir.NewFunc(mangledName)

	// Copy params from __new__ but skip self
	if len(newFn.Params) > 0 {
		wrapper.Func().Params = make([]hir.Param, len(newFn.Params)-1)
		for i := 1; i < len(newFn.Params); i++ {
			wrapper.Func().Params[i-1] = newFn.Params[i]
		}
	}
	wrapper.Func().RetType = "ptr"

	entry := hir.NewBlock("entry")

	// Calculate total size with concrete types
	totalSize := 0
	if baseCls != nil {
		for _, field := range baseCls.Fields {
			concreteType := substituteType(field.Type, subst)
			totalSize += getSize(concreteType)
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

	entry.Stmts = append(entry.Stmts, &hir.Ret{Val: instancePtr})
	wrapper.Func().Blocks = []*hir.Block{entry}

	return []*hir.Func{wrapper.Func(), newFn}
}

// lowerMonomorphizedMethods generates specialized methods for a class instantiation.
func lowerMonomorphizedMethods(cd *ast.ClassDecl, mangledName string, subst map[string]types.T, info *check.Info, baseCls *types.Class, src []byte) []*hir.Func {
	var funcs []*hir.Func

	for _, method := range cd.Methods {
		methodName := method.Name.Name

		// Skip __new__ (handled by constructor)
		if methodName == "__new__" {
			continue
		}

		// Lower method as function
		fn := LowerFuncFromDecl(method, info, src)

		// Specialized name: MangledName_methodName
		fn.Name = fmt.Sprintf("%s_%s", mangledName, methodName)

		// Get the method's semantic type from the class to apply proper substitution
		var methodType *types.Func
		if baseCls != nil {
			if m, ok := baseCls.Methods[methodName]; ok {
				methodType = m
			} else if m, ok := baseCls.Properties[methodName]; ok {
				methodType = m
			}
		}

		// Apply type substitution to return type
		if methodType != nil && methodType.Ret != nil {
			concreteRet := substituteType(methodType.Ret, subst)
			fn.RetType = lowerType(concreteRet)
		}

		// Handle self injection and parameter types for instance methods
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

		if !isStatic && !isClassMethod {
			// Check if self is already present
			hasSelf := false
			selfIdx := -1
			for i, p := range fn.Params {
				if p.Name == "self" {
					hasSelf = true
					selfIdx = i
					break
				}
			}

			if !hasSelf {
				// Inject self parameter at the beginning
				fn.Params = append([]hir.Param{{Name: "self", Type: "ptr"}}, fn.Params...)
				selfIdx = 0
			}

			// Now substitute types for remaining parameters (after self)
			// methodType.Params[0] is self (cls), so skip it
			if methodType != nil && len(methodType.Params) > 1 {
				for i := 1; i < len(methodType.Params); i++ {
					paramType := substituteType(methodType.Params[i], subst)
					hirIdx := selfIdx + i // fn.Params[selfIdx+1] matches methodType.Params[1]
					if hirIdx < len(fn.Params) {
						fn.Params[hirIdx].Type = lowerType(paramType)
					}
				}
			}
		} else {
			// Static/class methods: substitute all parameter types
			if methodType != nil {
				for i := 0; i < len(methodType.Params) && i < len(fn.Params); i++ {
					paramType := substituteType(methodType.Params[i], subst)
					fn.Params[i].Type = lowerType(paramType)
				}
			}
		}

		// Also need to substitute types inside the function body (Load/Store operations)
		substituteHIRFuncBody(fn, subst, baseCls)

		funcs = append(funcs, fn)
	}

	return funcs
}

// substituteHIRFuncBody walks the HIR function body and substitutes types
// in Load, Store, and other operations that involve generic types.
func substituteHIRFuncBody(fn *hir.Func, subst map[string]types.T, baseCls *types.Class) {
	if baseCls == nil || len(subst) == 0 {
		return
	}

	// Determine the concrete type for field accesses
	// For Box<int>.val, the field type is T which becomes int
	var fieldTypes map[string]types.T
	if baseCls != nil {
		fieldTypes = make(map[string]types.T)
		for _, f := range baseCls.Fields {
			fieldTypes[f.Name] = substituteType(f.Type, subst)
		}
	}

	// Walk all blocks and statements
	for _, block := range fn.Blocks {
		for i, stmt := range block.Stmts {
			switch s := stmt.(type) {
			case *hir.Load:
				// If loading from a field of a generic class, use the concrete type
				// Check if DesiType is a TypeParam and substitute
				if tp, ok := s.DesiType.(*types.TypeParam); ok {
					if concrete, ok := subst[tp.Name]; ok {
						s.Type = lowerType(concrete)
						s.DesiType = concrete
					}
				} else if s.Type == "ptr" {
					// Check if this might be a generic field load
					// The DesiType from checker should have the correct info
					if dt, ok := s.DesiType.(types.T); ok {
						concrete := substituteType(dt, subst)
						if concrete != dt {
							s.Type = lowerType(concrete)
							s.DesiType = concrete
						}
					}
				}
				block.Stmts[i] = s

			case *hir.Ret:
				// Return value type is handled by function return type
				// The value itself is already lowered correctly in most cases

			case *hir.Call:
				// Check if this is a call to a generic method and substitute return type
				if s.Type == "ptr" {
					// Could check if this needs substitution
				}
			}
		}
	}
}

// substituteHIRType converts a HIR type string based on the substitution map.
// For example, "ptr" -> "i32" if T maps to int.
func substituteHIRType(hirType string, subst map[string]types.T) string {
	// If it's already a concrete type, return as-is
	if hirType != "ptr" {
		return hirType
	}

	// For now, we cannot do much because HIR types are strings, not linked to type params
	// The proper fix requires threading type info through the lowering
	// For MVP, we rely on type erasure and ptr being compatible

	// TODO: Full implementation would need to track which parameters/returns are type params
	return hirType
}

// substituteType applies the substitution map to a type.
func substituteType(t types.T, subst map[string]types.T) types.T {
	if t == nil {
		return nil
	}

	switch x := t.(type) {
	case *types.TypeParam:
		if concrete, ok := subst[x.Name]; ok {
			return concrete
		}
		return t
	case *types.List:
		return &types.List{Elem: substituteType(x.Elem, subst)}
	case *types.Dict:
		return &types.Dict{Key: substituteType(x.Key, subst), Val: substituteType(x.Val, subst)}
	case *types.Set:
		return &types.Set{Elem: substituteType(x.Elem, subst)}
	case *types.Func:
		params := make([]types.T, len(x.Params))
		for i, p := range x.Params {
			params[i] = substituteType(p, subst)
		}
		return &types.Func{Params: params, Ret: substituteType(x.Ret, subst), Variadic: x.Variadic}
	default:
		return t
	}
}
