package resolve

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// Info holds bindings and a module graph summary produced by Resolve.
type Info struct {
	// For Phase-1 we only need enough to inject names into the checker.
	// Keys are the local names bound in the importing module.
	Imports   map[string]*ast.Module // "import foo as bar" -> key "bar" (or "foo") => target module
	FromItems map[string]*ast.Module // "from foo import x as y" -> key "y" (or "x") => defining module

	Graph *Graph // edges: thisModule -> importedModule
}

// Resolve walks import statements in 'mod', uses 'ldr' to load targets,
// checks graph cycles/duplicates/alias conflicts, reports diags, and returns binding Info.
// NOTE: This initial skeleton only returns empty bindings; logic will be added next batch.
func Resolve(mod *ast.Module, ldr Loader) ([]diag.Diagnostic, *Info) {
	_ = ldr
	info := &Info{
		Imports:   map[string]*ast.Module{},
		FromItems: map[string]*ast.Module{},
		Graph:     NewGraph(),
	}
	var diags []diag.Diagnostic
	return diags, info
}
