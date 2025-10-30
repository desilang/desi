package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

// PopulateImportedFuncSigs scans top-level from-imports in 'mod' and, for each local name,
// populates Info.Funcs[local] with the exact typed signatures exported by the target module.
// If no exported signatures exist (or module failed to load), it still ensures an empty set
// exists for the local, preserving Phase-1 permissive behavior.
func PopulateImportedFuncSigs(mod *ast.Module, info *Info, rinfo *resolve.Info) {
	if mod == nil || info == nil || rinfo == nil {
		return
	}
	// find synthetic __top__ function
	var top *ast.FuncDecl
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "__top__" {
			top = fn
			break
		}
	}
	if top == nil || top.Body == nil {
		return
	}
	dotted := func(segs []string) string { return strings.Join(segs, ".") }

	for _, st := range top.Body.Stmts {
		fi, ok := st.(*ast.FromImportStmt)
		if !ok {
			continue
		}
		mpath := dotted(fi.Path)
		ex := rinfo.ModuleExports[mpath]
		for _, it := range fi.Items {
			// Determine the local binding name
			local := it.Name.Name
			if it.Alias != nil {
				local = it.Alias.Name
			}
			if local == "" {
				continue
			}
			// Ensure a set exists
			set, ok := info.Funcs[local]
			if !ok || set == nil {
				set = &OverloadSet{Name: local}
				info.Funcs[local] = set
			}
			// Append exported candidates, if any
			if ex != nil {
				name := it.Name.Name
				if name != "" {
					if cands, ok := ex.Funcs[name]; ok {
						for _, ft := range cands {
							set.Add(&FuncCand{Decl: nil, Type: types.FuncOf(ft.Params, ft.Ret)})
						}
					}
				}
			}
		}
	}
}
