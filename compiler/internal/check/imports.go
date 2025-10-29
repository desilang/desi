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
	// `import a.b [as x]`  -> bind local name as a value (module handle); not callable.
	for local := range info.Imports {
		top.Define(&Symbol{Name: local, Kind: SymVar})
	}
	// `from a.b import y [as z]` -> bind local name as a callable alias in Phase-1.
	// We don't yet have cross-module signatures, but marking it SymFunc lets typCall
	// take the Phase-1 permissive path (same-primitive passthrough).
	for local := range info.FromItems {
		top.Define(&Symbol{Name: local, Kind: SymFunc})
	}
}
