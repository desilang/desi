// closure_lower.go - Lower entry module and all imported modules to a single HIR module
package lower

import (
	"os"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/resolve"
)

// LowerModuleClosure lowers the entry module and all transitively imported modules.
// It returns a combined hir.Module with:
// - All functions from the entry module
// - All exported (pub) functions from imported modules (namespaced with module prefix)
// - Deduplicated built-in constructors (Option, Result)
func LowerModuleClosure(
	mod *ast.Module,
	info *check.Info,
	src []byte,
	loader resolve.Loader,
) *hir.Module {
	out := &hir.Module{Name: mod.File}
	emittedFuncs := make(map[string]bool) // Track emitted function names to avoid duplicates

	// Track built-in constructors (Option, Result) to avoid duplicates
	builtinConstructors := map[string]bool{
		"Option.Some":    false,
		"Option.Nothing": false,
		"Result.Ok":      false,
		"Result.Err":     false,
	}

	// 1. Lower the entry module first
	entryHIR := LowerModuleFromSource(mod, info, src)
	for _, fn := range entryHIR.Funcs {
		// Check for built-in constructors
		if _, isBuiltin := builtinConstructors[fn.Name]; isBuiltin {
			if !builtinConstructors[fn.Name] {
				out.Funcs = append(out.Funcs, fn)
				builtinConstructors[fn.Name] = true
			}
			continue
		}
		if !emittedFuncs[fn.Name] {
			out.Funcs = append(out.Funcs, fn)
			emittedFuncs[fn.Name] = true
		}
	}
	// Mark __top__ as emitted to prevent import modules from emitting it
	emittedFuncs["__top__"] = true

	// 2. Lower imported modules
	if info != nil && info.R != nil {
		// Process "import X" modules
		for localName, impMod := range info.R.Imports {
			if impMod == nil {
				continue
			}
			lowerImportedModule(impMod, localName, loader, out, emittedFuncs, builtinConstructors)
		}

		// Process "from X import Y" modules
		processedModules := make(map[string]bool)
		for _, impMod := range info.R.FromItems {
			if impMod == nil || processedModules[impMod.File] {
				continue
			}
			processedModules[impMod.File] = true
			// For from-imports, we use the module's base name as prefix
			modPath := extractModulePath(impMod.File)
			lowerImportedModule(impMod, modPath, loader, out, emittedFuncs, builtinConstructors)
		}
	}

	return out
}

// lowerImportedModule lowers an imported module and adds its pub functions to the output.
// Functions are namespaced with modulePrefix_ to avoid conflicts.
func lowerImportedModule(
	mod *ast.Module,
	modulePrefix string,
	loader resolve.Loader,
	out *hir.Module,
	emittedFuncs map[string]bool,
	builtinConstructors map[string]bool,
) {
	// Try to get source for the module
	var src []byte
	if mod.File != "" {
		// Try to read the file
		data, err := os.ReadFile(mod.File)
		if err == nil {
			src = data
		}
	}

	// We need to type-check the imported module to get proper type info
	// For now, use nil info since we don't have full type info for imports
	// TODO: Enhance to share type info across modules
	// Skip built-in enums (Option/Result) since they're already emitted from entry module
	impHIR := LowerModuleFromSourceWithOptions(mod, nil, src, LowerModuleOptions{SkipBuiltinEnums: true, IsImportedModule: true})

	for _, fn := range impHIR.Funcs {
		// Skip built-in constructors if already emitted
		if _, isBuiltin := builtinConstructors[fn.Name]; isBuiltin {
			if !builtinConstructors[fn.Name] {
				out.Funcs = append(out.Funcs, fn)
				builtinConstructors[fn.Name] = true
			}
			continue
		}

		// Skip non-public functions (check if the origin has Pub flag)
		if fn.Origin != nil {
			if fd, ok := fn.Origin.(*ast.FuncDecl); ok && !fd.Pub {
				continue
			}
		}

		// Skip synthetic functions like __top__
		if fn.Name == "__top__" {
			continue
		}

		// Determine the original (unmangled) function name.
		// Module functions with IsImportedModule=true get __desi$ prefix;
		// strip it to recover the original name for namespacing.
		originalName := fn.Name
		if strings.HasPrefix(fn.Name, "__desi$") {
			originalName = strings.TrimPrefix(fn.Name, "__desi$")
		}

		// Create namespaced function name using ORIGINAL name: modulePrefix_functionName
		// e.g., math_add for import math; math.add()
		namespacedName := modulePrefix + "_" + originalName

		// Check if we already have this function
		if emittedFuncs[namespacedName] {
			continue
		}

		// Emit with original name (mangled or not — module_lower.go already
		// emits unmangled aliases, so this covers both)
		if !emittedFuncs[fn.Name] {
			out.Funcs = append(out.Funcs, fn)
			emittedFuncs[fn.Name] = true
		}

		// Emit the namespaced version for qualified calls
		if !emittedFuncs[namespacedName] {
			clonedFn := cloneHIRFunc(fn, namespacedName)
			out.Funcs = append(out.Funcs, clonedFn)
			emittedFuncs[namespacedName] = true
		}
	}
}

// extractModulePath extracts the module name from a file path
// e.g., "/path/to/math/__mod.desi" -> "math"
// e.g., "/path/to/util/math.desi" -> "math"
func extractModulePath(filePath string) string {
	// Remove .desi extension
	base := strings.TrimSuffix(filePath, ".desi")

	// Remove __mod suffix if it's a package (unconditional TrimSuffix handles both cases)
	base = strings.TrimSuffix(base, "/__mod")

	// Get the last path component
	parts := strings.Split(base, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}

// cloneHIRFunc creates a copy of an HIR function with a new name
func cloneHIRFunc(fn *hir.Func, newName string) *hir.Func {
	cloned := &hir.Func{
		Name:    newName,
		Params:  make([]hir.Param, len(fn.Params)),
		RetType: fn.RetType,
		Blocks:  fn.Blocks, // Share blocks (they don't change)
		Origin:  fn.Origin,
	}
	copy(cloned.Params, fn.Params)
	return cloned
}
