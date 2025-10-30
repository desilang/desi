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
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd == nil || fd.Body == nil {
			continue
		}
		if fd.Name.Name != "__top__" {
			continue
		}
		for _, s := range fd.Body.Stmts {
			if imp, ok := s.(*ast.ImportStmt); ok && imp != nil {
				local := ""
				if imp.Alias != nil {
					local = imp.Alias.Name
				} else if n := len(imp.Path); n > 0 {
					local = imp.Path[n-1]
				}
				if local != "" {
					mpath := strings.Join(imp.Path, ".")
					m[local] = mpath
				}
			}
		}
		break // only one __top__ is expected
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
	set = &OverloadSet{Name: fe.Name.Name}
	for _, ft := range cands {
		set.Add(&FuncCand{Decl: nil, Type: ft})
	}
	return set, id, true
}
