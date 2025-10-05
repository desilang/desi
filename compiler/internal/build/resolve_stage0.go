package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parser"
)

/* ---------- Stage-0 (Go lexer) path ---------- */

func ResolveAndParse(entryPath string) (*ast.File, []error) {
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
	}
	rootDir := filepath.Dir(entryAbs)
	roots := buildSearchRoots(entryAbs)

	type unit struct {
		path string // absolute file path
		file *ast.File
	}
	var (
		errs   []error
		seen   = map[string]bool{} // absolute path → true
		stack  = []string{}        // for cycle diagnostics (abs paths)
		result = []*unit{}
	)

	var load func(absPath string)
	load = func(absPath string) {
		if seen[absPath] {
			return
		}
		// cycle check
		for _, on := range stack {
			if on == absPath {
				chain := append(append([]string{}, stack...), absPath)
				errs = append(errs, modErr{
					code:   "DME0001",
					title:  "import cycle",
					key:    "import_cycle",
					detail: formatChain(chain, rootDir),
				})
				return
			}
		}
		stack = append(stack, absPath)
		defer func() { stack = stack[:len(stack)-1] }()

		data, rerr := os.ReadFile(absPath)
		if rerr != nil {
			errs = append(errs, fmt.Errorf("read %s: %v", rel(rootDir, absPath), rerr))
			return
		}
		p := parser.New(string(data))
		f, perr := p.ParseFile()
		if perr != nil {
			// IMPORTANT: do not wrap parser errors; keep their typed diagnostics + spans.
			errs = append(errs, perr)
			return
		}

		// resolve imports (std.* resolves via any ancestor compiler/lib)
		seenLocal := map[string]bool{} // per-file duplicate-import detection

		// plain imports
		for _, imp := range f.Imports {
			path := strings.TrimSpace(imp.Path)
			if path == "" {
				continue
			}
			if seenLocal[path] {
				errs = append(errs, modErr{
					code:   "DMW0001",
					title:  "duplicate import",
					key:    "duplicate_import",
					detail: fmt.Sprintf("%q in %s", path, rel(rootDir, absPath)),
				})
				continue
			}
			seenLocal[path] = true

			target, tried := resolveModule(roots, path)
			if target == "" {
				errs = append(errs, modErr{
					code:  "DME0002",
					title: "cannot find module",
					key:   "missing_module",
					detail: fmt.Sprintf("%q (looked for: %s)", path, strings.Join(func(ss []string) []string {
						out := make([]string, len(ss))
						for i, s := range ss {
							out[i] = rel(rootDir, s)
						}
						return out
					}(tried), ", ")),
				})
				continue
			}
			load(mustAbs(target))
		}

		// from-imports (resolve by module name)
		for _, fi := range f.FromImports {
			path := strings.TrimSpace(fi.Module)
			if path == "" {
				continue
			}
			if seenLocal[path] {
				errs = append(errs, modErr{
					code:   "DMW0001",
					title:  "duplicate import",
					key:    "duplicate_import",
					detail: fmt.Sprintf("%q in %s", path, rel(rootDir, absPath)),
				})
				continue
			}
			seenLocal[path] = true

			target, tried := resolveModule(roots, path)
			if target == "" {
				errs = append(errs, modErr{
					code:  "DME0002",
					title: "cannot find module",
					key:   "missing_module",
					detail: fmt.Sprintf("%q (looked for: %s)", path, strings.Join(func(ss []string) []string {
						out := make([]string, len(ss))
						for i, s := range ss {
							out[i] = rel(rootDir, s)
						}
						return out
					}(tried), ", ")),
				})
				continue
			}
			load(mustAbs(target))
		}

		result = append(result, &unit{path: absPath, file: f})
		seen[absPath] = true
	}

	load(entryAbs)

	if len(errs) > 0 {
		return nil, errs
	}

	// Merge: entry first, then others.
	var merged ast.File
	merged.LocalFuncNames = map[string]bool{}
	for _, u := range result {
		if same(u.path, entryAbs) {
			merged.Pkg = u.file.Pkg
			merged.Imports = append(merged.Imports, u.file.Imports...)
			merged.FromImports = append(merged.FromImports, u.file.FromImports...)
			merged.Decls = append(merged.Decls, u.file.Decls...)
			// record local (entry) function names
			for _, d := range u.file.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok {
					merged.LocalFuncNames[fn.Name] = true
				}
			}
		}
	}
	for _, u := range result {
		if !same(u.path, entryAbs) {
			merged.Imports = append(merged.Imports, u.file.Imports...)
			merged.FromImports = append(merged.FromImports, u.file.FromImports...)
			merged.Decls = append(merged.Decls, u.file.Decls...)
			// deliberately do NOT add non-entry funcs to LocalFuncNames
		}
	}
	return &merged, nil
}
