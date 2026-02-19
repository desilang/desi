package check

import (
	"strings"

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
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "__top__" || fn.Body == nil {
			continue
		}
		for _, st := range fn.Body.Stmts {
			if im, ok := st.(*ast.ImportStmt); ok {
				if len(im.Path) > 0 {
					local := im.Path[len(im.Path)-1]
					if im.Alias != nil && im.Alias.Name != "" {
						local = im.Alias.Name
					}
					m[local] = strings.Join(im.Path, ".")
				}
			}
		}
	}
	return m
}

// moduleQualifiedOverloadSet returns an overload set for a callee of the form "mod.fn".
func (c *checker) moduleQualifiedOverloadSet(fe *ast.FieldExpr) (set *OverloadSet, base *ast.Ident, isImport bool) {
	if fe == nil || c == nil || c.info == nil {
		return nil, nil, false
	}
	id, ok := fe.X.(*ast.Ident)
	if !ok || id == nil {
		return nil, nil, false
	}
	// Recognize only if 'id' is a local import binding we recorded.
	mpath, ok := c.info.ImportPaths[id.Name]
	if !ok || mpath == "" || c.info.R == nil {
		return nil, id, false
	}
	// Mark the qualifier ident as resolved so unused-import lint sees a use.
	if sym := c.scope.Lookup(id.Name); sym != nil {
		c.info.Idents[id] = sym
	}
	// Pull candidates from resolver exports.
	ex := c.info.R.ModuleExports[mpath]
	if ex == nil {
		return &OverloadSet{Name: fe.Name.Name}, id, true
	}
	cands := ex.Funcs[fe.Name.Name]
	modesTab := ex.FuncModes[fe.Name.Name]
	namesTab := ex.ParamNames[fe.Name.Name]
	metaTab := ex.FuncExtern[fe.Name.Name]
	defaultsTab := ex.FuncDefaults[fe.Name.Name]
	declsTab := ex.FuncDecls[fe.Name.Name]

	set = &OverloadSet{Name: fe.Name.Name}
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
		var pnames []string
		if i < len(namesTab) {
			pnames = namesTab[i]
		}
		ext := false
		if i < len(metaTab) {
			ext = metaTab[i].Extern
		}
		var defaults []bool
		if i < len(defaultsTab) {
			defaults = defaultsTab[i]
		}
		var decl *ast.FuncDecl
		if i < len(declsTab) {
			decl = declsTab[i]
		}
		set.Add(&FuncCand{
			Decl:       nil, // Keep nil to avoid borrow checker regressions
			Type:       ft,
			Modes:      modes,
			Extern:     ext,
			ParamNames: cloneNames(pnames),
			Defaults:   cloneBools(defaults),
			ModuleDecl: decl, // Stored for lowerer's overload dispatch
		})
	}
	return set, id, true
}

// cloneNames is a tiny helper to avoid aliasing exported slices.
func cloneNames(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// cloneBools is a tiny helper to avoid aliasing exported default masks.
func cloneBools(in []bool) []bool {
	if len(in) == 0 {
		return nil
	}
	out := make([]bool, len(in))
	copy(out, in)
	return out
}

// hasCands reports whether an overload set is non-nil and non-empty.
func hasCands(s *OverloadSet) bool { return s != nil && len(s.Cands) > 0 }
