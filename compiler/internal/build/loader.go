package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexbridge"
	"github.com/desilang/desi/compiler/internal/parser"
)

// ResolveAndParse loads the entry file, resolves imports recursively, and returns
// a single merged *ast.File that concatenates all Decls (entry first, then deps).
// Import rules (Stage-0):
//   - import paths like "foo.bar" resolve to "<dir>/foo/bar.desi"
//   - imports starting with "std." are ignored (runtime-provided)
//   - cycles are detected and reported
//   - duplicate loads are skipped
func ResolveAndParse(entryPath string) (*ast.File, []error) {
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
	}
	rootDir := filepath.Dir(entryAbs)

	type unit struct {
		path string // absolute file path
		file *ast.File
	}
	var (
		errs   []error
		seen   = map[string]bool{} // absolute path → true
		stack  = []string{}        // for cycle diagnostics
		result = []*unit{}
	)

	var load func(absPath string)
	load = func(absPath string) {
		if seen[absPath] {
			return
		}
		// cycle check: if it's already on the stack, it's a cycle
		for _, on := range stack {
			if on == absPath {
				errs = append(errs, fmt.Errorf("import cycle detected involving %s", rel(rootDir, absPath)))
				return
			}
		}
		stack = append(stack, absPath)
		defer func() { stack = stack[:len(stack)-1] }()

		data, err := os.ReadFile(absPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %v", rel(rootDir, absPath), err))
			return
		}
		p := parser.New(string(data))
		f, err := p.ParseFile()
		if err != nil {
			errs = append(errs, fmt.Errorf("parse %s: %v", rel(rootDir, absPath), err))
			return
		}

		// resolve imports
		for _, imp := range f.Imports {
			path := imp.Path
			// ignore std.* for Stage-0
			if strings.HasPrefix(path, "std.") {
				continue
			}
			relPath := strings.ReplaceAll(path, ".", string(filepath.Separator)) + ".desi"
			target := filepath.Join(rootDir, relPath)
			if !fileExists(target) {
				errs = append(errs, fmt.Errorf("import %q → %s not found (from %s)",
					path, rel(rootDir, target), rel(rootDir, absPath)))
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

	// Merge: entry file first, then others in load order (which is DFS post-order).
	// Ensure entry is first by stable partition.
	var merged ast.File
	merged.Pkg = nil
	merged.Imports = nil
	merged.Decls = nil

	// Put entry first
	for _, u := range result {
		if same(u.path, entryAbs) {
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}
	// Then all others
	for _, u := range result {
		if !same(u.path, entryAbs) {
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}

	return &merged, nil
}

// ResolveAndParseWith is like ResolveAndParse, but callers supply a loader that
// returns a parser.TokenSource for each absolute file path. This enables using
// the Stage-1 Desi lexer (via bridge) while reusing the same import resolution.
func ResolveAndParseWith(entryPath string, loader func(absPath string) (parser.TokenSource, error)) (*ast.File, []error) {
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
	}
	rootDir := filepath.Dir(entryAbs)

	type unit struct {
		path string // absolute file path
		file *ast.File
	}
	var (
		errs   []error
		seen   = map[string]bool{} // absolute path → true
		stack  = []string{}        // for cycle diagnostics
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
				errs = append(errs, fmt.Errorf("import cycle detected involving %s", rel(rootDir, absPath)))
				return
			}
		}
		stack = append(stack, absPath)
		defer func() { stack = stack[:len(stack)-1] }()

		// Use the provided token-source loader (e.g., Desi lexer bridge)
		src, lerr := loader(absPath)
		if lerr != nil {
			errs = append(errs, fmt.Errorf("load %s: %v", rel(rootDir, absPath), lerr))
			return
		}
		p := parser.NewFromSource(src)
		f, perr := p.ParseFile()
		if perr != nil {
			errs = append(errs, fmt.Errorf("parse %s: %v", rel(rootDir, absPath), perr))
			return
		}

		// resolve imports
		for _, imp := range f.Imports {
			path := imp.Path
			if strings.HasPrefix(path, "std.") {
				continue
			}
			relPath := strings.ReplaceAll(path, ".", string(filepath.Separator)) + ".desi"
			target := filepath.Join(rootDir, relPath)
			if !fileExists(target) {
				errs = append(errs, fmt.Errorf("import %q → %s not found (from %s)",
					path, rel(rootDir, target), rel(rootDir, absPath)))
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

	// Merge (entry first)
	var merged ast.File
	merged.Pkg = nil
	merged.Imports = nil
	merged.Decls = nil

	for _, u := range result {
		if same(u.path, entryAbs) {
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}
	for _, u := range result {
		if !same(u.path, entryAbs) {
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}

	return &merged, nil
}

// ResolveAndParseMaybeDesi chooses between Go lexer (default) and the Desi lexer bridge.
// keepTmp controls whether to retain generated bridge artifacts under gen/tmp.
// verbose enables bridge logging/diagnostics.
func ResolveAndParseMaybeDesi(entryPath string, useDesiLexer bool, keepTmp, verbose bool) (*ast.File, []error) {
	if !useDesiLexer {
		return ResolveAndParse(entryPath)
	}
	loader := func(absPath string) (parser.TokenSource, error) {
		// lexbridge returns a lexer.Source which is compatible with parser.TokenSource.
		return lexbridge.NewSourceFromFileOpts(absPath, keepTmp, verbose)
	}
	return ResolveAndParseWith(entryPath, loader)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
func mustAbs(p string) string {
	a, _ := filepath.Abs(p)
	return a
}
func same(a, b string) bool {
	aa, _ := filepath.EvalSymlinks(a)
	bb, _ := filepath.EvalSymlinks(b)
	if aa == "" {
		aa = a
	}
	if bb == "" {
		bb = b
	}
	return aa == bb
}
func rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return r
}
