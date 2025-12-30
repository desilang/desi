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
		Classes:      map[string]*types.Class{},
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
				Name:         qualifiedName,
				Constructors: []*types.Func{},
				Fields:       []types.Field{},
				Methods:      map[string]*types.Func{},
				IsNested:     true,
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
				// Include self (the nested class type) as first param
				// This matches how checkClass processes methods - expr_field.go strips self when creating bound methods
				params := []types.T{nestedType}
				for _, p := range method.Params {
					if p.Name.Name == "self" {
						continue // Skip explicit self param, we already added nestedType
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
				nestedType.Methods[methodName] = methodFunc
			}

			// Add default constructor if none found
			if len(nestedType.Constructors) == 0 {
				nestedType.Constructors = append(nestedType.Constructors, types.FuncOf(nil, nestedType, false))
			}

			out.Classes[qualifiedName] = nestedType
		}
	}
	return out
}
