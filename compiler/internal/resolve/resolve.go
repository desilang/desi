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

	// Phase-2: typed export surface per module (functions only for now),
	// keyed by dotted module path (e.g., "math", "util.math").
	ModuleExports map[string]*Exports
}

// dotted joins path segments with dots.
func dotted(segments []string) string { return strings.Join(segments, ".") }

// diagAt constructs a simple Diagnostic in the "module" domain.
func diagAt(codeID string, span diag.Span, msg string) diag.Diagnostic {
	return diag.Diagnostic{
		CodeID:  codeID,
		Domain:  "module",
		Title:   "",
		Message: msg,
		Primary: diag.Label{Span: span, Primary: true},
	}
}

// Resolve walks top-level imports in 'mod', loads targets via 'ldr', binds local
// names, and records edges in the module graph. It also collects typed function
// exports for each imported module and validates that from-items exist in that
// export surface (emitting DME0003 when they do not).
func Resolve(mod *ast.Module, ldr Loader) ([]diag.Diagnostic, *Info) {
	info := &Info{
		Imports:       map[string]*ast.Module{},
		FromItems:     map[string]*ast.Module{},
		Graph:         NewGraph(),
		ModuleExports: map[string]*Exports{},
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
			// Load the module; surface any parse diags but keep going.
			tmod, pdiags, _ := ldr.Load(mpath)
			if len(pdiags) > 0 {
				diags = append(diags, pdiags...)
			}
			// Collect exports once per module path (if load succeeded).
			if tmod != nil {
				if _, seen := info.ModuleExports[mpath]; !seen {
					info.ModuleExports[mpath] = CollectExports(tmod)
				}
			}
			local := ""
			if s.Alias != nil {
				local = s.Alias.Name
			} else if len(s.Path) > 0 {
				local = s.Path[len(s.Path)-1]
			}
			if local != "" {
				info.Imports[local] = tmod
			}
			info.Graph.AddEdge(mod.File, mpath)

		case *ast.FromImportStmt:
			mpath := dotted(s.Path)
			// Load target module; append parse diags if any.
			tmod, pdiags, _ := ldr.Load(mpath)
			if len(pdiags) > 0 {
				diags = append(diags, pdiags...)
			}
			// Collect/export surface for validation and later checker use.
			var ex *Exports
			if tmod != nil {
				if cached, seen := info.ModuleExports[mpath]; seen {
					ex = cached
				} else {
					ex = CollectExports(tmod)
					info.ModuleExports[mpath] = ex
				}
			}
			for _, it := range s.Items {
				local := it.Name.Name
				if it.Alias != nil {
					local = it.Alias.Name
				}
				if local != "" {
					info.FromItems[local] = tmod
				}
				// Validate that the requested item exists among exported funcs.
				if ex != nil {
					name := it.Name.Name
					if name != "" {
						if len(ex.Funcs[name]) == 0 {
							msg := mpath + " has no exported '" + name + "'"
							diags = append(diags, diagAt("DME0003", it.Span, msg))
						}
					}
				}
			}
			info.Graph.AddEdge(mod.File, mpath)
		}
	}

	return diags, info
}
