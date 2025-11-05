package resolve

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// ExternMeta carries per-overload extern metadata aligned with Exports.Funcs[name][i].
type ExternMeta struct {
	Extern bool   // true if this overload is declared @extern(...)
	ABI    string // e.g., "C"
	Link   string // optional link hint (e.g., "m")
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

	// FuncExtern is index-aligned with Funcs[name]: extern metadata per overload.
	FuncExtern map[string][]ExternMeta
}

// CollectExports walks a parsed module and returns its exported function signatures.
//
// Rules implemented here:
//   - Consider only TOP-LEVEL function declarations (methods/nested funcs ignored).
//   - Include only functions where every parameter has an explicit type annotation
//     resolvable via types.FromName AND the return type is explicitly annotated.
//   - Visibility: only `pub def` are exported.
//   - FFI: detect @extern("C"[, "linklib"]) decorator and record metadata aligned to overloads.
func CollectExports(mod *ast.Module) *Exports {
	out := &Exports{
		Funcs:      map[string][]*types.Func{},
		FuncModes:  map[string][][]ast.ParamMode{},
		FuncExtern: map[string][]ExternMeta{},
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
				params[i] = t
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

		ft := types.FuncOf(params, rt)
		name := fn.Name.Name
		out.Funcs[name] = append(out.Funcs[name], ft)

		// Collect parameter modes aligned with this overload.
		modes := make([]ast.ParamMode, len(fn.Params))
		for i := range fn.Params {
			modes[i] = fn.Params[i].Mode
		}
		out.FuncModes[name] = append(out.FuncModes[name], modes)

		// ---- FFI extern metadata (index-aligned) ----
		meta := ExternMeta{}
		for _, dec := range fn.Decorators {
			if dec.Name.Name != "extern" {
				continue
			}
			// Expect @extern("C") or @extern("C", "m")
			if len(dec.Args) >= 1 {
				if s, ok := dec.Args[0].(*ast.StrLit); ok {
					meta.ABI = s.Value
					if meta.ABI == "C" {
						meta.Extern = true
					}
				}
			}
			if len(dec.Args) >= 2 {
				if s, ok := dec.Args[1].(*ast.StrLit); ok {
					meta.Link = s.Value
				}
			}
			// Additional arguments (symbol override, etc.) can be added later.
			// For M9B, we keep it simple and ignore unexpected arg shapes.
			break
		}
		out.FuncExtern[name] = append(out.FuncExtern[name], meta)
	}
	return out
}
