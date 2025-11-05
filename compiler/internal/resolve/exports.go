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
//   - FFI: detect @extern(<string>[, <string>]) decorator and record metadata aligned to overloads.
//     Tier-0: ABI is set to "C" if the first arg is any string literal; link is marked as present if a second string exists.
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
			// Tier-0: if first arg is a string literal, assume ABI "C" and mark extern.
			if len(dec.Args) >= 1 {
				if _, ok := dec.Args[0].(*ast.StrLit); ok {
					meta.ABI = "C"
					meta.Extern = true
				}
			}
			// If second arg is a string literal, mark link hint as present with a placeholder.
			if len(dec.Args) >= 2 {
				if _, ok := dec.Args[1].(*ast.StrLit); ok {
					meta.Link = "__present__" // placeholder: presence-only in M9B
				}
			}
			// Stop after the first extern decorator, if multiple.
			break
		}
		out.FuncExtern[name] = append(out.FuncExtern[name], meta)
	}
	return out
}
