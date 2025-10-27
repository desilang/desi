package resolve

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// Info holds bindings and a module graph summary produced by Resolve.
type Info struct {
	// Local name -> module that defines it (Phase-1: just module-level)
	Imports   map[string]*ast.Module // "import a.b [as x]" => key x or "b"
	FromItems map[string]*ast.Module // "from a.b import y [as z]" => key z or "y"
	Graph     *Graph                 // edges: thisModule -> importedModule (dotted)
}

// dotted joins path segments with dots.
func dotted(segments []string) string { return strings.Join(segments, ".") }

// Resolve walks top-level imports in 'mod', loads targets via 'ldr', binds local
// names, and records edges in the module graph. Diagnostics (unknown module,
// cycles, dupes, alias conflicts, unused) are intentionally deferred to Phase-1b.
func Resolve(mod *ast.Module, ldr Loader) ([]diag.Diagnostic, *Info) {
	info := &Info{
		Imports:   map[string]*ast.Module{},
		FromItems: map[string]*ast.Module{},
		Graph:     NewGraph(),
	}
	var diags []diag.Diagnostic

	// Find the synthetic top function "__top__" and walk its Block.
	var top *ast.FuncDecl
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "__top__" {
			top = fn
			break
		}
	}
	if top == nil || top.Body == nil {
		// Nothing to resolve; return empty info.
		return diags, info
	}

	for _, st := range top.Body.Stmts {
		switch s := st.(type) {
		case *ast.ImportStmt:
			mpath := dotted(s.Path)
			tmod, _, err := ldr.Load(mpath)
			if err != nil {
				// Phase-1b: map to DME0002
				continue
			}
			// Local binding name: alias or last path segment
			local := ""
			if s.Alias != nil {
				local = s.Alias.Name
			} else if len(s.Path) > 0 {
				local = s.Path[len(s.Path)-1]
			}
			if local != "" {
				info.Imports[local] = tmod
			}
			// Graph edge: current -> target (use dotted path)
			info.Graph.AddEdge(mod.File, mpath)

		case *ast.FromImportStmt:
			mpath := dotted(s.Path)
			tmod, _, err := ldr.Load(mpath)
			if err != nil {
				// Phase-1b: map to DME0002
				continue
			}
			for _, it := range s.Items {
				local := it.Name.Name
				if it.Alias != nil {
					local = it.Alias.Name
				}
				if local != "" {
					info.FromItems[local] = tmod
				}
			}
			info.Graph.AddEdge(mod.File, mpath)
		}
	}

	return diags, info
}
