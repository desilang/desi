package resolve

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// Exports summarizes the public API we care about for cross-module type checking.
// Phase-2 scope: functions only (no classes/structs/enums/consts yet).
type Exports struct {
	// Funcs maps a function name to its typed overloads.
	// Only overloads with fully annotated params and return type are included.
	Funcs map[string][]*types.Func
}

// CollectExports walks a parsed module and returns its exported function signatures.
//
// Phase-2 rules implemented here:
//   - Consider only TOP-LEVEL function declarations (methods/nested funcs ignored).
//   - Include only functions where every parameter has an explicit type annotation
//     resolvable via types.FromName AND the return type is explicitly annotated.
//   - Visibility: only `pub def` are exported.
func CollectExports(mod *ast.Module) *Exports {
	out := &Exports{Funcs: map[string][]*types.Func{}}
	if mod == nil {
		return out
	}
	for _, d := range mod.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue // not a function
		}
		if !fn.Pub {
			continue // NEW: export gate requires pub
		}

		// All params must be annotated and resolvable.
		params := make([]types.T, len(fn.Params))
		okTypes := true
		for i, p := range fn.Params {
			if p.Type == nil {
				okTypes = false
				break
			}
			pt, ok := types.FromName(p.Type.Name)
			if !ok {
				okTypes = false
				break
			}
			params[i] = pt
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
	}
	return out
}
