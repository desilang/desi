package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// computeImportPaths builds local import name -> dotted module mapping
// by scanning the hoisted __top__ block for ImportStmt at top-level.
func computeImportPaths(mod *ast.Module) map[string]string {
	m := make(map[string]string)
	if mod == nil {
		return m
	}
	for _, d := range mod.Decls {
		if im, ok := d.(*ast.ImportStmt); ok {
			mpath := dotted(im.Path)
			local := ""
			if im.Alias != nil {
				local = im.Alias.Name
			} else if len(im.Path) > 0 {
				local = im.Path[len(im.Path)-1]
			}
			if local != "" {
				m[local] = mpath
			}
		}
	}
	return m
}

// moduleQualifiedOverloadSet returns exported overloads for a call of the
// form 'mod.fn(...)' where 'mod' is a local import binding.
func (c *checker) moduleQualifiedOverloadSet(fe *ast.FieldExpr) (*OverloadSet, *ast.Ident, bool) {
	id, ok := fe.X.(*ast.Ident)
	if !ok || id == nil {
		return nil, nil, false
	}
	// Map local import binding to its module path
	if c.info == nil || c.info.R == nil {
		return nil, nil, false
	}
	mpath := c.info.ImportPaths[id.Name]
	if mpath == "" {
		return nil, nil, false
	}
	ex := c.info.R.ModuleExports[mpath]
	set := &OverloadSet{Name: fe.Name.Name}
	if ex == nil {
		return set, id, true
	}
	cands := ex.Funcs[fe.Name.Name]
	modesTab := ex.FuncModes[fe.Name.Name]
	metaTab := ex.FuncExtern[fe.Name.Name]
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
	return set, id, true
}
