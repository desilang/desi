package resolve

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
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
				// augment with re-exports declared in the imported module itself
				info.ModuleExports[mpath] = reexportIntoExports(info.ModuleExports[mpath], tmod, ldr, info, &diags)
			}
			local := ""
			if s.Alias != nil {
				local = s.Alias.Name
			} else if len(s.Path) > 0 {
				local = s.Path[len(s.Path)-1]
			}
			if local != "" && tmod != nil {
				info.Imports[local] = tmod
			}
			info.Graph.AddEdge(mod.File, mpath)

		case *ast.FromImportStmt:
			mpath := dotted(s.Path)
			// Load the module and capture its exports.
			tmod, pdiags, _ := ldr.Load(mpath)
			if len(pdiags) > 0 {
				diags = append(diags, pdiags...)
			}
			var ex *Exports
			if tmod != nil {
				if cur, seen := info.ModuleExports[mpath]; seen {
					ex = cur
				} else {
					ex = CollectExports(tmod)
					info.ModuleExports[mpath] = ex
				}
				// augment with re-exports declared in that module as well
				info.ModuleExports[mpath] = reexportIntoExports(ex, tmod, ldr, info, &diags)
				ex = info.ModuleExports[mpath] // update after re-exports
			}

			// Handle wildcard import: from X import *
			if s.Star {
				if tmod != nil && ex != nil {
					// Import all exported functions
					for name := range ex.Funcs {
						info.FromItems[name] = tmod
					}
				}
				info.Graph.AddEdge(mod.File, mpath)
				continue // Skip normal item processing
			}

			for _, it := range s.Items {
				local := it.Name.Name
				if it.Alias != nil {
					local = it.Alias.Name
				}
				if local != "" && tmod != nil {
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

// reexportIntoExports augments 'ex' with function exports re-exported by 'mod' via
// top-level "from X import a [as b]" statements. It loads X using 'ldr' and copies
// typed candidates into 'ex' under the local binding name (alias if present).
func reexportIntoExports(ex *Exports, mod *ast.Module, ldr Loader, info *Info, diags *[]diag.Diagnostic) *Exports {
	if mod == nil {
		return ex
	}
	// Ensure ex maps are non-nil
	if ex == nil {
		ex = &Exports{
			Funcs:        map[string][]*types.Func{},
			FuncModes:    map[string][][]ast.ParamMode{},
			ParamNames:   map[string][][]string{},
			FuncExtern:   map[string][]ExternMeta{},
			FuncDefaults: map[string][][]bool{},
		}
	}
	// Find the synthetic top function to read its statements.
	var top *ast.FuncDecl
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "__top__" {
			top = fn
			break
		}
	}
	if top == nil || top.Body == nil {
		return ex
	}
	for _, st := range top.Body.Stmts {
		fi, ok := st.(*ast.FromImportStmt)
		if !ok {
			continue
		}
		subpath := dotted(fi.Path)
		submod, pdiags, _ := ldr.Load(subpath)
		if len(pdiags) > 0 {
			*diags = append(*diags, pdiags...)
		}
		if submod == nil {
			continue
		}
		// Ensure we have exports for submodule.
		subEx, ok := info.ModuleExports[subpath]
		if !ok || subEx == nil {
			subEx = CollectExports(submod)
			info.ModuleExports[subpath] = subEx
		}
		// Copy requested items into ex under the local binding name.
		for _, it := range fi.Items {
			name := it.Name.Name
			local := name
			if it.Alias != nil {
				local = it.Alias.Name
			}
			if name == "" || local == "" {
				continue
			}
			cands := subEx.Funcs[name]
			if len(cands) == 0 {
				continue
			}
			modesTab := subEx.FuncModes[name]
			metaTab := subEx.FuncExtern[name]
			namesTab := subEx.ParamNames[name]
			defaultsTab := subEx.FuncDefaults[name]

			// Append (not overwrite) to allow multiple sources providing overloads.
			ex.Funcs[local] = append(ex.Funcs[local], cands...)
			ex.FuncModes[local] = append(ex.FuncModes[local], modesTab...)
			ex.FuncExtern[local] = append(ex.FuncExtern[local], metaTab...)
			ex.ParamNames[local] = append(ex.ParamNames[local], namesTab...)
			ex.FuncDefaults[local] = append(ex.FuncDefaults[local], defaultsTab...)
		}
	}
	return ex
}
