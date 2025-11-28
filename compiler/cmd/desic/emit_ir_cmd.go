package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/term"
)

// init() runs before main(). We intercept "emit-ir" here and exit after handling it.
// This avoids modifying the existing main.go.
func init() {
	if len(os.Args) < 2 || os.Args[1] != "emit-ir" {
		return
	}

	file, roots, err := parseEmitArgs(os.Args[2:])
	if err != nil {
		term.Eprintln("emit-ir error:", err)
		term.Flush()
		os.Exit(2)
	}

	src, err := os.ReadFile(file)
	if err != nil {
		term.Eprintln("emit-ir:", err)
		term.Flush()
		os.Exit(2)
	}

	// Parse (no resolve/check here; emit-ir is a Tier-0 demo tool)
	mod, pdiags := parse.ParseFile(file, src)
	if len(pdiags) > 0 {
		for _, d := range pdiags {
			// Render directly to stderr with a default (zero-value) theme.
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		term.Flush()
		os.Exit(2)
	}

	// NEW: resolve imports with a filesystem loader and register LLVM
	// function signature overrides from the full import closure.
	var loader resolve.Loader
	if roots != "" {
		rs := splitRoots(roots)
		if len(rs) > 0 && rs[0] != "" {
			loader = resolve.NewFSLoaderMulti(rs)
		}
	}
	if loader == nil {
		loader = resolve.NewMemLoader(nil)
	}

	// Run full type checker (M14: needed for struct/trait info)
	res := check.CheckWithLoader(mod, loader)
	if len(res.Diags) > 0 {
		// Keep it simple: if check produced diagnostics, surface them and bail.
		const maxDiags = 20
		limit := len(res.Diags)
		if limit > maxDiags {
			limit = maxDiags
		}
		for i := 0; i < limit; i++ {
			res.Diags[i].RenderTTY(os.Stderr, diag.Theme{})
		}
		if extra := len(res.Diags) - limit; extra > 0 {
			term.Eprintln("…", extra, "more errors suppressed")
		}
		term.Flush()
		os.Exit(2)
	}

	// Register LLVM signatures from imports (Tier-0 compat)
	registerImportClosureSigs(res.Info.R)

	// Lower the entry module to HIR
	hm := lower.LowerModuleFromSource(mod, res.Info, src)
	if hm == nil || len(hm.Funcs) == 0 {
		term.Eprintln("emit-ir:", filepath.Base(file)+": no functions to lower")
		term.Flush()
		os.Exit(2)
	}

	// Collect all HIR modules (entry + imports)
	allModules := []*hir.Module{hm}
	allASTs := []*ast.Module{mod}

	// Track processed modules to avoid duplicates
	processed := make(map[string]bool)
	processed[filepath.Base(file)] = true // mark entry module as processed

	// Process all imported modules
	importedPaths := collectImportedModulePaths(res.Info.R)
	for _, impPath := range importedPaths {
		if processed[impPath] {
			continue
		}
		processed[impPath] = true

		// Load and lower the imported module
		impHIR, _, impAST, err := loadAndLowerModule(impPath, loader, res.Info)
		if err != nil {
			// Skip modules that fail to load (they might be extern/built-in)
			continue
		}
		if impHIR != nil {
			allModules = append(allModules, impHIR)
		}
		if impAST != nil {
			allASTs = append(allASTs, impAST)
		}
	}

	// Build textual LLVM module. Mark async wrapper names if their poll exists.
	lm := llvm.NewModule(filepath.Base(file))
	markAsyncWrappers(lm, hm)

	// Inject param/ret textual types from surface annotations in all modules
	for _, astMod := range allASTs {
		injectUserFuncSigs(astMod)
	}

	// Collect all struct and enum declarations from AST modules
	var allStructDecls []*ast.StructDecl
	var allEnumDecls []*ast.EnumDecl
	for _, astMod := range allASTs {
		for _, decl := range astMod.Decls {
			if structDecl, ok := decl.(*ast.StructDecl); ok {
				allStructDecls = append(allStructDecls, structDecl)
			} else if enumDecl, ok := decl.(*ast.EnumDecl); ok {
				allEnumDecls = append(allEnumDecls, enumDecl)
			}
		}
	}

	// Emit struct/enum type definitions before functions
	lm.EmitTypeDefs(allStructDecls, allEnumDecls, res.Info)

	// Mark all functions as defined to avoid unnecessary declarations
	for _, hirMod := range allModules {
		for _, f := range hirMod.Funcs {
			lm.MarkDefined(f.Name)
		}
	}

	// Emit all functions from all modules
	for _, hirMod := range allModules {
		for _, f := range hirMod.Funcs {
			lm.EmitFunc(f)
		}
	}

	fmt.Print(lm.IR())
	term.Flush()
	os.Exit(0)
}

// parseEmitArgs accepts flags in any order after `emit-ir` and returns (file, roots).
// Supports: -I ROOTS, -I=ROOTS, and "--" to end flags.
// Usage: desic emit-ir [-I ROOTS] <file.desi>
func parseEmitArgs(argv []string) (string, string, error) {
	var file string
	var roots string

	sawSep := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			sawSep = true
			continue
		}
		if !sawSep && strings.HasPrefix(a, "-") {
			switch {
			case a == "-I":
				if i+1 >= len(argv) {
					return "", "", fmt.Errorf("missing value for -I")
				}
				roots = argv[i+1]
				i++
			case strings.HasPrefix(a, "-I="):
				roots = strings.TrimPrefix(a, "-I=")
			default:
				return "", "", fmt.Errorf("unknown flag %q", a)
			}
			continue
		}
		// positional: first non-flag is the entry file
		if file == "" {
			file = a
		}
	}

	if file == "" {
		return "", "", fmt.Errorf("usage: desic emit-ir [-I ROOTS] <file.desi>")
	}
	return file, roots, nil
}

// findMainFunc is kept for possible future use; currently unused.
func findMainFunc(m *ast.Module) *ast.FuncDecl {
	for _, d := range m.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			if fd.Name.Name == "main" && fd.Body != nil {
				return fd
			}
		}
	}
	return nil
}

// markAsyncWrappers scans HIR for pairs "<name>" and "<name>$poll" and marks "<name>"
// as an async wrapper in the LLVM module so calls to it are emitted as 'call ptr'.
func markAsyncWrappers(lm *llvm.Module, hm *hir.Module) {
	seenPoll := map[string]bool{}
	for _, f := range hm.Funcs {
		if strings.HasSuffix(f.Name, "$poll") {
			base := strings.TrimSuffix(f.Name, "$poll")
			seenPoll[base] = true
		}
	}
	for _, f := range hm.Funcs {
		if seenPoll[f.Name] {
			lm.MarkAsyncWrapper(f.Name)
		}
	}
}

// injectUserFuncSigs scans the AST for fn params/ret and registers textual LLVM types.
// Unrecognized/omitted types fall back to existing Tier-0 defaults (ptr/i32).
func injectUserFuncSigs(mod *ast.Module) {
	if mod == nil {
		return
	}
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		params := make([]string, len(fd.Params))
		for i := range fd.Params {
			if fd.Params[i].Type == nil {
				params[i] = "" // keep default (ptr)
				continue
			}
			params[i] = llvmTypeFromSurface(fd.Params[i].Type.Name)
		}
		ret := ""
		if fd.RetType != nil {
			ret = llvmTypeFromSurface(fd.RetType.Name)
		}
		// Package-level override (no Module struct changes needed).
		llvm.SetFuncSig(fd.Name.Name, ret, params)
	}
	// M14: Register struct/class constructors as returning ptr
	for _, d := range mod.Decls {
		var name string
		if s, ok := d.(*ast.StructDecl); ok {
			name = s.Name.Name
		} else if c, ok := d.(*ast.ClassDecl); ok {
			name = c.Name.Name
		}
		if name != "" {
			llvm.SetFuncSig(name, "ptr", nil)
			// Also register the default to_str method
			llvm.SetFuncSig(name+"_to_str", "ptr", nil)
		}
	}
}

// llvmTypeFromSurface maps simple surface type names to textual LLVM IR types.
// This is the same Tier-0 mapping used for per-module annotations.
func llvmTypeFromSurface(name string) string {
	switch name {
	case "bool":
		return "i1"
	case "f32":
		return "float"
	case "f64", "float":
		return "double"
	case "usize", "isize":
		return "i64"
	case "str":
		return "ptr"
	case "i8", "i16", "i32", "i64", "i128":
		return name
	case "u8":
		return "i8"
	case "u16":
		return "i16"
	case "u32":
		return "i32"
	case "u64":
		return "i64"
	case "u128":
		return "i128"
	default:
		return "" // unknown → leave default
	}
}

// registerImportClosureSigs installs LLVM func signature overrides using the
// typed export surfaces collected by resolve.Resolve across the import closure.
func registerImportClosureSigs(info *resolve.Info) {
	if info == nil || info.ModuleExports == nil {
		return
	}
	for _, ex := range info.ModuleExports {
		if ex == nil {
			continue
		}
		for name, overloads := range ex.Funcs {
			if len(overloads) == 0 {
				continue
			}
			ft := overloads[0]
			if ft == nil {
				continue
			}
			ret, params := llvm.LowerFuncSignature(ft)
			llvm.SetFuncSig(name, ret, params)
			// We deliberately use the first overload only for Tier-0; true
			// overload-aware lowering will come later with a real call resolver.
		}
	}
}

// collectImportedModulePaths extracts all unique module paths from the import closure.
// It returns paths in no particular order.
func collectImportedModulePaths(rinfo *resolve.Info) []string {
	if rinfo == nil || rinfo.ModuleExports == nil {
		return nil
	}

	// Use a map to deduplicate paths
	seen := make(map[string]bool)
	for path := range rinfo.ModuleExports {
		seen[path] = true
	}

	// Convert to slice
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}

	return paths
}

// loadAndLowerModule loads a module by its dotted path and lowers it to HIR.
// Returns (HIR module, source bytes, error). The source is returned for signature injection.
func loadAndLowerModule(path string, loader resolve.Loader, info *check.Info) (*hir.Module, []byte, *ast.Module, error) {
	// Load the module using the loader
	mod, diags, err := loader.Load(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load module %s: %w", path, err)
	}
	if len(diags) > 0 {
		return nil, nil, nil, fmt.Errorf("parse errors in module %s", path)
	}

	// Read the source file for lowering
	// The loader returns a parsed module but we need the source bytes
	// For now, we'll try to read the file again if we can determine the path
	// This is a limitation of the current loader interface

	// For now, pass nil as source - lower will handle it
	// (LowerModuleFromSource can work without source for most cases)
	var src []byte

	// Lower to HIR
	hm := lower.LowerModuleFromSource(mod, info, src)

	return hm, src, mod, nil
}
