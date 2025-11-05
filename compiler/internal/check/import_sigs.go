package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/resolve"
)

// PopulateImportedFuncSigs scans top-level from-imports in 'mod' and, for each local name,
// populates Info.Funcs[local] with the exact typed signatures exported by the target module.
// If no exported signatures exist (or module failed to load), it still ensures an empty set
// exists for the local, preserving Phase-1 permissive behavior.
func PopulateImportedFuncSigs(mod *ast.Module, info *Info, rinfo *resolve.Info) {
	if mod == nil || info == nil || rinfo == nil {
		return
	}
	// Only care about 'from ... import ...' here.
	for _, d := range mod.Decls {
		fimp, ok := d.(*ast.FromImportStmt)
		if !ok {
			continue
		}
		mpath := dotted(fimp.Path)
		ex, ok := rinfo.ModuleExports[mpath]
		if !ok {
			// ensure empty set so resolution still works (Phase-1 permissive path)
			for _, it := range fimp.Items {
				local := it.Name.Name
				if it.Alias != nil {
					local = it.Alias.Name
				}
				if strings.TrimSpace(local) == "" {
					continue
				}
				if _, ok2 := info.Funcs[local]; !ok2 {
					info.Funcs[local] = &OverloadSet{Name: local}
				}
			}
			continue
		}
		for _, it := range fimp.Items {
			name := it.Name.Name
			local := name
			if it.Alias != nil {
				local = it.Alias.Name
			}
			if strings.TrimSpace(local) == "" {
				continue
			}
			set, ok2 := info.Funcs[local]
			if !ok2 || set == nil {
				set = &OverloadSet{Name: local}
				info.Funcs[local] = set
			}
			cands := ex.Funcs[name]
			modesTab := ex.FuncModes[name]
			metaTab := ex.FuncExtern[name]
			for i, ft := range cands {
				var modes []ast.ParamMode
				if i < len(modesTab) {
					modes = modesTab[i]
				}
				if modes == nil {
					modes = make([]ast.ParamMode, len(ft.Params))
					for j := range modes {
						modes[j] = ast.ParamMove
					}
				}
				extern := false
				if i < len(metaTab) {
					extern = metaTab[i].Extern
				}
				set.Add(&FuncCand{Decl: nil, Type: ft, Modes: modes, Extern: extern})
			}
		}
	}
}

// dotted joins ident path segments with '.'
func dotted(xs []*ast.Ident) string {
	parts := make([]string, len(xs))
	for i, id := range xs {
		parts[i] = id.Name
	}
	return strings.Join(parts, ".")
}
