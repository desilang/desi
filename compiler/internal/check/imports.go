package check

import (
	"github.com/desilang/desi/compiler/internal/resolve"
)

// injectImports places resolver-bound names into the top scope
// so imported identifiers behave like locals during type checking.
func injectImports(top *Scope, info *resolve.Info) {
	if info == nil || top == nil {
		return
	}
	// import a.b [as x] -> x (or b)
	for local := range info.Imports {
		top.Define(&Symbol{Name: local, Kind: SymVar})
	}
	// from a.b import y [as z] -> z (or y)
	for local := range info.FromItems {
		top.Define(&Symbol{Name: local, Kind: SymVar})
	}
}
