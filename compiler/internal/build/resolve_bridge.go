package build

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parsebridge"
)

/* ---------- Stage-1 (Desi parser bridge runner) ---------- */

func ResolveAndParseWithParserBridge(entryFile string, useExternal bool, bin string, keepTmp, verbose bool) (*ast.File, []error) {
	entryAbs, err := filepath.Abs(entryFile)
	if err != nil {
		return nil, []error{fmt.Errorf("abs(%s): %v", entryFile, err)}
	}
	rootDir := filepath.Dir(entryAbs)
	roots := buildSearchRoots(entryAbs)

	type unit struct {
		path string
		file *ast.File
	}

	var (
		errs   []error
		seen   = map[string]bool{}
		stack  []string
		result []*unit
	)

	var parseOne func(absPath string) *ast.File
	parseOne = func(absPath string) *ast.File {
		var js []byte
		var err error
		if useExternal {
			js, err = parsebridge.Run(absPath, bin, verbose)
		} else {
			js, err = parsebridge.BuildAndRunJSON(absPath, keepTmp, verbose)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("bridge parse %s: %v", rel(rootDir, absPath), err))
			return nil
		}
		f, uerr := ast.UnmarshalFileJSON(js)
		if uerr != nil {
			errs = append(errs, fmt.Errorf("bridge AST JSON invalid for %s: %v", rel(rootDir, absPath), uerr))
			return nil
		}
		return f
	}

	var load func(absPath string)
	load = func(absPath string) {
		if seen[absPath] {
			return
		}
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

		if !fileExists(absPath) {
			errs = append(errs, fmt.Errorf("read %s: not found", rel(rootDir, absPath)))
			return
		}

		f := parseOne(absPath)
		if f == nil {
			return
		}

		seenLocal := map[string]bool{}

		// plain imports
		for _, im := range f.Imports {
			path := strings.TrimSpace(im.Path)
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

		// NEW: from-imports
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

	var merged ast.File
	for _, u := range result {
		if same(u.path, entryAbs) {
			merged.Pkg = u.file.Pkg
			merged.Imports = append(merged.Imports, u.file.Imports...)
			merged.FromImports = append(merged.FromImports, u.file.FromImports...)
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}
	for _, u := range result {
		if !same(u.path, entryAbs) {
			merged.Imports = append(merged.Imports, u.file.Imports...)
			merged.FromImports = append(merged.FromImports, u.file.FromImports...)
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}
	return &merged, nil
}
