package resolve

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// ExternMeta carries per-overload extern metadata aligned with Exports.Funcs[name][i].
type ExternMeta struct {
	Extern bool   // true if this overload is declared @extern(...)
	ABI    string // e.g., "C" (Tier-0: assume C if a string is present)
	Link   string // optional link hint presence (non-empty means provided)
	Symbol string // optional symbol override (reserved for future use)
}

// Exports summarizes the public API we care about for cross-module type checking.
// Phase-2/FFI scope: functions only (no classes/structs/enums/consts yet).
type Exports struct {
	// Funcs maps a function name to its typed overloads.
	// Only overloads with fully annotated params and return type are included.
	Funcs map[string][]*types.Func

	// FuncModes is index-aligned with Funcs[name]: one []ParamMode per overload.
	// This carries the callee-declared parameter passing modes for each exported function.
	FuncModes map[string][][]ast.ParamMode

	// ParamNames is index-aligned with Funcs[name]: one []string per overload.
	// ParamNames[name][i][j] is the source-level parameter name for overload i, parameter j.
	ParamNames map[string][][]string

	// FuncExtern is index-aligned with Funcs[name]: extern metadata per overload.
	FuncExtern map[string][]ExternMeta

	// FuncDefaults is index-aligned with Funcs[name]: one []bool per overload.
	// FuncDefaults[name][i][j] is true if parameter j of overload i has a default value.
	FuncDefaults map[string][][]bool

	// FuncDecls is index-aligned with Funcs[name]: the AST FuncDecl per overload.
	// Used by the lowerer to extract the underlying @extern function from pub def wrappers.
	FuncDecls map[string][]*ast.FuncDecl

	// Classes maps a class name to its type information.
	// Allows "from module import ClassName" to work with class types.
	Classes map[string]*types.Class

	// Globals maps exported global variable names to their types.
	// Allows "from module import CONST" to work.
	Globals map[string]types.T
}

// collectClasses extracts public class declarations from a module and populates out.Classes
// with fully-typed class information including fields, constructors, and methods.
// This must be called BEFORE function collection so that function return types can reference
// the same class instances that have method information populated.
func collectClasses(mod *ast.Module, out *Exports) {
	for _, d := range mod.Decls {
		cls, ok := d.(*ast.ClassDecl)
		if !ok || !cls.Pub {
			continue
		}
		name := cls.Name.Name

		// Create a Class type for export
		classType := &types.Class{
			Name:         name,
			Constructors: []*types.Func{},
			Fields:       []types.Field{},
			Methods:      map[string]*types.Func{},
			Dunders:      map[string]*types.Func{},
			Properties:   map[string]*types.Func{},
		}

		// Extract fields from class
		for _, field := range cls.Fields {
			if field.Type == nil {
				continue
			}
			fieldType, ok := types.FromName(field.Type.Name)
			if !ok {
				continue
			}
			classType.Fields = append(classType.Fields, types.Field{
				Name:  field.Name.Name,
				Type:  fieldType,
				IsPub: field.Pub,
				IsMut: field.Mut,
			})
		}

		// Extract methods from class (including __new__ and dunders)
		for _, method := range cls.Methods {
			methodName := method.Name.Name

			// Handle __new__ as constructor
			if methodName == "__new__" {
				params := make([]types.T, 0, len(method.Params))
				for _, p := range method.Params {
					if p.Name.Name == "self" || p.Name.Name == "cls" {
						continue
					}
					if p.Type == nil {
						continue
					}
					if t, ok := types.FromName(p.Type.Name); ok {
						params = append(params, t)
					} else {
						// Unknown type (e.g., generic param T) - treat as Any
						params = append(params, types.Any)
					}
				}
				returnType := classType
				constructorFunc := types.FuncOf(params, returnType, false)
				classType.Constructors = append(classType.Constructors, constructorFunc)
				continue
			}

			// Build the method function type (including private methods for dunders)
			params := make([]types.T, 0, len(method.Params))
			for _, p := range method.Params {
				if p.Name.Name == "self" {
					continue
				}
				if p.Type == nil {
					continue
				}
				t, ok := types.FromName(p.Type.Name)
				if !ok {
					// Unknown type (e.g., generic type param T) - treat as Any
					t = types.Any
				}
				params = append(params, t)
			}
			var retType types.T = types.None
			if method.RetType != nil {
				if t, ok := types.FromName(method.RetType.Name); ok {
					retType = t
				}
			}
			methodFunc := types.FuncOf(params, retType, false)
			methodFunc.IsPub = method.Pub

			// Store dunders in Dunders map, others in Methods map
			if len(methodName) > 4 && methodName[:2] == "__" && methodName[len(methodName)-2:] == "__" {
				classType.Dunders[methodName] = methodFunc
			} else if method.Pub {
				classType.Methods[methodName] = methodFunc
			}
		}

		// If no __new__ found, add a default zero-arg constructor
		if len(classType.Constructors) == 0 {
			classType.Constructors = append(classType.Constructors, types.FuncOf(nil, classType, false))
		}

		out.Classes[name] = classType
	}
}

// CollectExports walks a parsed module and returns its exported function signatures.
//
// Rules implemented here:
//   - Consider only TOP-LEVEL function declarations (methods/nested funcs ignored).
//   - Include only functions where every parameter has an explicit type annotation
//     resolvable via types.FromName AND the return type is explicitly annotated.
//   - Visibility: only `pub def` are exported.
//   - FFI: detect @extern(<string>[, <string>]) decorator and record metadata aligned to overloads.
//     Tier-0: ABI is set to "C" if the first arg is any string literal; link is marked as present if a second string exists.
func CollectExports(mod *ast.Module) *Exports {
	out := &Exports{
		Funcs:        map[string][]*types.Func{},
		FuncModes:    map[string][][]ast.ParamMode{},
		ParamNames:   map[string][][]string{},
		FuncExtern:   map[string][]ExternMeta{},
		FuncDefaults: map[string][][]bool{},
		FuncDecls:    map[string][]*ast.FuncDecl{},

		Classes: map[string]*types.Class{},
		Globals: map[string]types.T{},
	}
	if mod == nil {
		return out
	}

	// ========================================================================
	// FIRST: Collect public class declarations (so function return types can reference them)
	// ========================================================================
	collectClasses(mod, out)

	// ========================================================================
	// FIRST-B: Collect struct declarations (so function return types can reference them)
	// ========================================================================
	structTypes := map[string]*types.Struct{}
	for _, d := range mod.Decls {
		sd, ok := d.(*ast.StructDecl)
		if !ok {
			continue
		}
		fields := make([]types.Field, len(sd.Fields))
		allResolved := true
		for i, f := range sd.Fields {
			var ft types.T
			if f.Type != nil {
				if t, ok := types.FromName(f.Type.Name); ok {
					ft = t
				} else {
					allResolved = false
					break
				}
			} else {
				allResolved = false
				break
			}
			fields[i] = types.Field{
				Name:  f.Name.Name,
				Type:  ft,
				IsPub: f.Pub,
			}
		}
		if !allResolved {
			continue
		}
		structTypes[sd.Name.Name] = &types.Struct{
			Name:   sd.Name.Name,
			Fields: fields,
		}
	}

	// resolveTypeName resolves an AST TypeName to a types.T, handling generic
	// containers (list[T], dict[K,V], set[T]), tuples, unions, and simple types.
	var resolveTypeName func(tn *ast.TypeName) (types.T, bool)
	resolveTypeName = func(tn *ast.TypeName) (types.T, bool) {
		if tn == nil {
			return nil, false
		}

		// Handle tuple types: (int, str, ...)
		if len(tn.TupleTypes) > 0 {
			elems := make([]types.T, len(tn.TupleTypes))
			for i, tt := range tn.TupleTypes {
				t, ok := resolveTypeName(tt)
				if !ok {
					return nil, false
				}
				elems[i] = t
			}
			return types.TupleOf(elems...), true
		}

		// Handle union types: int|str|none
		if len(tn.UnionTypes) > 0 {
			// For now, treat unions as Any (the type system handles them at check time)
			return types.Any, true
		}

		// Handle generic containers: list[T], dict[K,V], set[T], Option[T], Result[T,E]
		if len(tn.Params) > 0 {
			switch tn.Name {
			case "list":
				if len(tn.Params) == 1 {
					elem, ok := resolveTypeName(tn.Params[0])
					if !ok {
						return nil, false
					}
					return types.ListOf(elem), true
				}
			case "dict":
				if len(tn.Params) == 2 {
					k, ok := resolveTypeName(tn.Params[0])
					if !ok {
						return nil, false
					}
					v, ok := resolveTypeName(tn.Params[1])
					if !ok {
						return nil, false
					}
					return types.DictOf(k, v), true
				}
			case "set":
				if len(tn.Params) == 1 {
					elem, ok := resolveTypeName(tn.Params[0])
					if !ok {
						return nil, false
					}
					return types.SetOf(elem), true
				}
			case "Option":
				if len(tn.Params) == 1 {
					elem, ok := resolveTypeName(tn.Params[0])
					if !ok {
						return nil, false
					}
					return types.OptionOf(elem), true
				}
			case "Result":
				if len(tn.Params) == 2 {
					okT, ok1 := resolveTypeName(tn.Params[0])
					errT, ok2 := resolveTypeName(tn.Params[1])
					if !ok1 || !ok2 {
						return nil, false
					}
					return types.ResultOf(okT, errT), true
				}
			}
		}

		// Simple type: try built-in types first
		if t, ok := types.FromName(tn.Name); ok {
			return t, true
		}
		// Then try classes exported from this module (with full method info)
		if cls, ok := out.Classes[tn.Name]; ok {
			return cls, true
		}
		// Then try structs defined in this module
		if st, ok := structTypes[tn.Name]; ok {
			return st, true
		}
		return nil, false
	}

	// ========================================================================
	// SECOND: Collect public function declarations
	// ========================================================================
	for _, d := range mod.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue // not a function
		}
		if !fn.Pub {
			continue // export gate requires pub
		}

		// All params must be annotated and resolvable.
		params := make([]types.T, len(fn.Params))
		okTypes := true
		for i, p := range fn.Params {
			if p.Type == nil {
				okTypes = false
				break
			}
			if t, ok := resolveTypeName(p.Type); ok {
				// If this is a variadic parameter, wrap in list[T]
				if p.Variadic {
					params[i] = types.ListOf(t)
				} else {
					params[i] = t
				}
			} else {
				okTypes = false
				break
			}
		}
		if !okTypes {
			continue
		}
		// Return type must be annotated and resolvable.
		if fn.RetType == nil {
			continue
		}
		rt, ok := resolveTypeName(fn.RetType)
		if !ok {
			continue
		}

		variadic := false
		for _, p := range fn.Params {
			if p.Variadic {
				variadic = true
			}
		}

		ft := types.FuncOf(params, rt, variadic)
		name := fn.Name.Name
		out.Funcs[name] = append(out.Funcs[name], ft)

		// Parameter modes aligned with this overload.
		modes := make([]ast.ParamMode, len(fn.Params))
		for i := range fn.Params {
			modes[i] = fn.Params[i].Mode
		}
		out.FuncModes[name] = append(out.FuncModes[name], modes)

		// Parameter names aligned with this overload.
		pnames := make([]string, len(fn.Params))
		for i := range fn.Params {
			pnames[i] = fn.Params[i].Name.Name
		}
		out.ParamNames[name] = append(out.ParamNames[name], pnames)

		// Default-argument mask aligned with this overload.
		defaults := make([]bool, len(fn.Params))
		for i, p := range fn.Params {
			if p.Default != nil {
				defaults[i] = true
			}
		}
		out.FuncDefaults[name] = append(out.FuncDefaults[name], defaults)

		// ---- FFI extern metadata (index-aligned) ----
		meta := ExternMeta{}
		for _, dec := range fn.Decorators {
			if dec.Name.Name != "extern" {
				continue
			}
			// Tier-0: if first arg is a string literal, assume ABI "C" and mark extern.
			if len(dec.Args) >= 1 {
				if _, ok := dec.Args[0].(*ast.StrLit); ok {
					meta.ABI = "C"
					meta.Extern = true
				}
			}
			// If second arg is a string literal, mark link hint as present.
			if len(dec.Args) >= 2 {
				if _, ok := dec.Args[1].(*ast.StrLit); ok {
					meta.Link = "__present__" // presence-only
				}
			}
			break
		}
		out.FuncExtern[name] = append(out.FuncExtern[name], meta)
		out.FuncDecls[name] = append(out.FuncDecls[name], fn)
	}

	// Handle nested classes (collectClasses already populated out.Classes with top-level classes)
	for _, d := range mod.Decls {
		cls, ok := d.(*ast.ClassDecl)
		if !ok || !cls.Pub {
			continue
		}
		name := cls.Name.Name
		classType := out.Classes[name]
		if classType == nil {
			continue // Should not happen, but safety check
		}

		// Export public nested classes with qualified name (Container.Inner)
		for _, nested := range cls.Nested {
			if !nested.Pub {
				continue // Skip private nested classes
			}
			qualifiedName := name + "." + nested.Name.Name
			nestedType := &types.Class{
				Name:          qualifiedName,
				Constructors:  []*types.Func{},
				Fields:        []types.Field{},
				Methods:       map[string]*types.Func{},
				StaticMethods: map[string]*types.Func{},
				Constants:     map[string]*types.ClassConstant{},
				IsNested:      true,
			}

			// Extract nested class fields
			for _, field := range nested.Fields {
				if field.Type == nil {
					continue
				}
				fieldType, ok := types.FromName(field.Type.Name)
				if !ok {
					continue
				}
				nestedType.Fields = append(nestedType.Fields, types.Field{
					Name:  field.Name.Name,
					Type:  fieldType,
					IsPub: field.Pub,
					IsMut: field.Mut,
				})
			}

			// Extract nested class methods
			for _, method := range nested.Methods {
				methodName := method.Name.Name
				if methodName == "__new__" {
					// Handle constructor
					params := make([]types.T, 0, len(method.Params))
					for _, p := range method.Params {
						if p.Name.Name == "self" || p.Name.Name == "cls" {
							continue
						}
						if p.Type == nil {
							continue
						}
						if t, ok := types.FromName(p.Type.Name); ok {
							params = append(params, t)
						}
					}
					constructorFunc := types.FuncOf(params, nestedType, false)
					nestedType.Constructors = append(nestedType.Constructors, constructorFunc)
					continue
				}
				if !method.Pub {
					continue
				}

				// Check if this is a static method (has @staticmethod decorator) BEFORE building params
				isStatic := false
				for _, dec := range method.Decorators {
					if dec.Name.Name == "staticmethod" {
						isStatic = true
						break
					}
				}

				// Only include self for instance methods, not static methods
				var params []types.T
				if !isStatic {
					// Instance method: include self (the nested class type) as first param
					params = []types.T{nestedType}
				} else {
					// Static method: no self parameter
					params = []types.T{}
				}

				for _, p := range method.Params {
					if p.Name.Name == "self" {
						continue // Skip explicit self param, we already added nestedType for instance methods
					}
					if p.Type == nil {
						continue
					}
					if t, ok := types.FromName(p.Type.Name); ok {
						params = append(params, t)
					}
				}
				var retType types.T = types.None
				if method.RetType != nil {
					if t, ok := types.FromName(method.RetType.Name); ok {
						retType = t
					}
				}
				methodFunc := types.FuncOf(params, retType, false)
				methodFunc.IsPub = true
				if isStatic {
					nestedType.StaticMethods[methodName] = methodFunc
				} else {
					nestedType.Methods[methodName] = methodFunc
				}
			}

			// Export nested class constants
			for _, constant := range nested.Constants {
				if !constant.IsPub {
					continue
				}
				var constType types.T = types.Any
				if constant.Type != nil {
					if t, ok := types.FromName(constant.Type.Name); ok {
						constType = t
					}
				}
				nestedType.Constants[constant.Name.Name] = &types.ClassConstant{
					Name:  constant.Name.Name,
					Type:  constType,
					Value: constant.Value, // Include Value for lowering to emit constant value
					IsPub: constant.IsPub,
				}
			}

			// Add default constructor if none found
			if len(nestedType.Constructors) == 0 {
				nestedType.Constructors = append(nestedType.Constructors, types.FuncOf(nil, nestedType, false))
			}

			out.Classes[qualifiedName] = nestedType
		}
	}

	// NEW: Collect exported globals from __top__
	for _, d := range mod.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "__top__" || fn.Body == nil {
			continue
		}
		for _, s := range fn.Body.Stmts {
			ls, ok := s.(*ast.LetStmt)
			if !ok || !ls.Pub {
				continue
			}
			// Resolve type
			var t types.T = types.Any
			if ls.Type != nil {
				if tt, ok := types.FromName(ls.Type.Name); ok {
					t = tt
				}
			}
			// Add to exports
			out.Globals[ls.Name.Name] = t
		}
	}

	return out
}
