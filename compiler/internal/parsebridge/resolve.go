package parsebridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parser"
)

func resolveAndParseLocal(rootDir, entryPath string) (*ast.File, []error) {
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
	}

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

	var load func(absPath string)
	load = func(absPath string) {
		if seen[absPath] {
			return
		}
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
		f, perr := p.ParseFile()
		if perr != nil {
			errs = append(errs, fmt.Errorf("parse %s: %v", rel(rootDir, absPath), perr))
			return
		}
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

	var merged ast.File
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
