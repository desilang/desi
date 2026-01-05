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
	Imports       map[string]*ast.Module // "import a.b [as x]" => key x or "b"
	FromItems     map[string]*ast.Module // "from a.b import y [as z]" => key z or "y"
	FromItemPaths map[string]string      // local name -> qualified path (e.g., "Inner" -> "Container.Inner")
	Graph         *Graph                 // edges: thisModule -> importedModule (dotted)

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

// stdPrefix is the reserved namespace prefix for stdlib.
const stdPrefix = "std."

// loadModule loads a module, handling std.* prefix and relative imports.
// Returns the module, diagnostics, actual path, and whether it was stdlib-only.
func loadModule(mpath string, ldr Loader, relative bool, span diag.Span) (*ast.Module, []diag.Diagnostic, string, bool) {
	var warnings []diag.Diagnostic

	// Check for reserved "std" namespace (user trying to import "std" module)
	if mpath == "std" {
		warnings = append(warnings, diagAt("DME0009", span, "'std' is a reserved namespace"))
		return nil, warnings, mpath, false
	}

	// Check for std. prefix - load from stdlib only
	if strings.HasPrefix(mpath, stdPrefix) {
		actualPath := strings.TrimPrefix(mpath, stdPrefix)
		if sl, ok := ldr.(StdlibLoader); ok {
			mod, diags, _ := sl.LoadStdlib(actualPath)
			return mod, diags, actualPath, true
		}
		// Fall back to regular load if not a StdlibLoader
		mod, diags, _ := ldr.Load(actualPath)
		return mod, diags, actualPath, true
	}

	// Regular local-first load
	mod, diags, _ := ldr.Load(mpath)

	// Check for shadow warning: local module found AND stdlib has same name
	// Only warn if LOCAL module exists (not if only stdlib has it)
	if mod != nil && !relative {
		if sl, ok := ldr.(StdlibLoader); ok && sl.HasLocalModule(mpath) && sl.HasStdlibModule(mpath) {
			warnings = append(warnings, diag.Diagnostic{
				CodeID:  "DME0010",
				Domain:  "module",
				Message: "'" + mpath + "' shadows stdlib module",
				Primary: diag.Label{Span: span, Primary: true},
			})
		}
	}

	diags = append(diags, warnings...)
	return mod, diags, mpath, false
}

// Resolve walks top-level imports in 'mod', loads targets via 'ldr', binds local
// names, and records edges in the module graph. It also collects typed function
// exports for each imported module and validates that from-items exist in that
// export surface (emitting DME0003 when they do not).
func Resolve(mod *ast.Module, ldr Loader) ([]diag.Diagnostic, *Info) {
	info := &Info{
		Imports:       map[string]*ast.Module{},
		FromItems:     map[string]*ast.Module{},
		FromItemPaths: map[string]string{},
		Graph:         NewGraph(),
		ModuleExports: map[string]*Exports{},
	}
	var diags []diag.Diagnostic
	visited := map[string]bool{} // track visited modules for cycle detection

	// Normalize file path to module name (e.g., "/path/foo.desi" -> "foo")
	srcModule := mod.File
	if srcModule != "" {
		// Extract basename and remove .desi extension
		base := srcModule
		if idx := strings.LastIndex(srcModule, "/"); idx >= 0 {
			base = srcModule[idx+1:]
		}
		if strings.HasSuffix(base, ".desi") {
			base = strings.TrimSuffix(base, ".desi")
		}
		srcModule = base
	}
	if srcModule == "" {
		srcModule = "main"
	}

	resolveImportsRecursive(mod, srcModule, ldr, info, &diags, visited)
	return diags, info
}

// resolveImportsRecursive resolves imports for a module and recursively resolves imported modules
func resolveImportsRecursive(mod *ast.Module, srcModule string, ldr Loader, info *Info, diags *[]diag.Diagnostic, visited map[string]bool) {
	if visited[srcModule] {
		return // Already processed
	}
	visited[srcModule] = true

	// Find the synthetic top function "__top__" and walk its Block.
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

	for _, st := range top.Body.Stmts {
		switch s := st.(type) {
		case *ast.ImportStmt:
			mpath := dotted(s.Path)
			// Load the module (handles std.* prefix and relative imports)
			tmod, pdiags, actualPath, _ := loadModule(mpath, ldr, s.Relative, s.Span)
			if len(pdiags) > 0 {
				*diags = append(*diags, pdiags...)
			}
			// Collect exports once per module path (if load succeeded).
			if tmod != nil {
				if _, seen := info.ModuleExports[actualPath]; !seen {
					info.ModuleExports[actualPath] = CollectExports(tmod)
				}
				// augment with re-exports declared in the imported module itself
				info.ModuleExports[actualPath] = reexportIntoExports(info.ModuleExports[actualPath], tmod, ldr, info, diags)
				// Recursively resolve imports from this module
				resolveImportsRecursive(tmod, actualPath, ldr, info, diags, visited)
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
			info.Graph.AddEdge(srcModule, actualPath)

		case *ast.FromImportStmt:
			mpath := dotted(s.Path)
			// Load the module (handles std.* prefix and relative imports)
			tmod, pdiags, actualPath, _ := loadModule(mpath, ldr, s.Relative, s.Span)
			if len(pdiags) > 0 {
				*diags = append(*diags, pdiags...)
			}
			var ex *Exports
			if tmod != nil {
				if cur, seen := info.ModuleExports[actualPath]; seen {
					ex = cur
				} else {
					ex = CollectExports(tmod)
					info.ModuleExports[actualPath] = ex
				}
				// augment with re-exports declared in that module as well
				info.ModuleExports[actualPath] = reexportIntoExports(ex, tmod, ldr, info, diags)
				ex = info.ModuleExports[actualPath] // update after re-exports
				// Recursively resolve imports from this module
				resolveImportsRecursive(tmod, actualPath, ldr, info, diags, visited)
			}

			// Handle wildcard import: from X import *
			if s.Star {
				if tmod != nil && ex != nil {
					// Import all exported functions
					for name := range ex.Funcs {
						info.FromItems[name] = tmod
					}
					// Import all exported classes
					for name := range ex.Classes {
						info.FromItems[name] = tmod
					}
				}
				info.Graph.AddEdge(srcModule, actualPath)
				continue // Skip normal item processing
			}

			for _, it := range s.Items {
				local := it.Name.Name
				if it.Alias != nil {
					local = it.Alias.Name
				}
				if local != "" && tmod != nil {
					info.FromItems[local] = tmod
					// Store qualified path for nested class lookup (e.g., "Inner" -> "Container.Inner")
					if len(it.Path) > 1 {
						info.FromItemPaths[local] = strings.Join(it.Path, ".")
					} else {
						info.FromItemPaths[local] = it.Name.Name
					}
				}
				// Validate that the requested item exists among exported funcs or classes.
				if ex != nil {
					// For nested imports like Container.Item, check the first segment
					var rootName string
					if len(it.Path) > 0 {
						rootName = it.Path[0] // Use first segment (e.g., "Container")
					} else {
						rootName = it.Name.Name
					}
					if rootName != "" {
						if len(ex.Funcs[rootName]) == 0 && ex.Classes[rootName] == nil {
							msg := actualPath + " has no exported '" + rootName + "'"
							*diags = append(*diags, diagAt("DME0003", it.Span, msg))
						}
					}
				}
			}
			info.Graph.AddEdge(srcModule, actualPath)
		}
	}
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
			Classes:      map[string]*types.Class{},
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
			if len(cands) > 0 {
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

			// Also re-export classes (e.g., from sync.mutex import Mutex)
			if cls, ok := subEx.Classes[name]; ok {
				ex.Classes[local] = cls
			}
		}
	}
	return ex
}
