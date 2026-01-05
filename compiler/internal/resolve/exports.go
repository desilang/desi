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

	// Classes maps a class name to its type information.
	// Allows "from module import ClassName" to work with class types.
	Classes map[string]*types.Class

	// Globals maps exported global variable names to their types.
	// Allows "from module import CONST" to work.
	Globals map[string]types.T
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

		Classes: map[string]*types.Class{},
		Globals: map[string]types.T{},
	}
	if mod == nil {
		return out
	}
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
			if t, ok := types.FromName(p.Type.Name); ok {
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
		rt, ok := types.FromName(fn.RetType.Name)
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
	}

	// Collect public class declarations
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

		// Extract methods from class (including __new__)
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

			// Skip non-public methods
			if !method.Pub {
				continue
			}

			// Build the method function type
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
			methodFunc.IsPub = true // Mark as public for cross-module access
			classType.Methods[methodName] = methodFunc
		}

		// If no __new__ found, add a default zero-arg constructor
		if len(classType.Constructors) == 0 {
			classType.Constructors = append(classType.Constructors, types.FuncOf(nil, classType, false))
		}

		out.Classes[name] = classType

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
