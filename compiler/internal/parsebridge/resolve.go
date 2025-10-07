package parsebridge

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/build"
	"github.com/desilang/desi/compiler/internal/parser"
)

// resolveAndParseLocal resolves imports starting from entryPath using the unified
// resolver, then parses the resulting plan and merges ASTs: entry first, then deps.
// - rootDir is kept for signature compatibility; resolver discovers roots automatically.
// - Any module-domain diagnostics (not-found, cycles) are returned as errors and abort parsing.
func resolveAndParseLocal(_rootDir, entryPath string) (*ast.File, []error) {
	plan, mdiags, rerr := build.ResolveEntry(entryPath, build.ResolveOptions{})
	if rerr != nil {
		return nil, []error{fmt.Errorf("resolve: %v", rerr)}
	}
	if len(mdiags) > 0 {
		errs := make([]error, 0, len(mdiags))
		for _, d := range mdiags {
			errs = append(errs, d)
		}
		return nil, errs
	}

	entryAbs := filepath.Clean(plan.Entry.File)
	var (
		entryDecls []ast.Decl
		depDecls   []ast.Decl
	)

	for _, u := range plan.Deps {
		data, err := os.ReadFile(u.File)
		if err != nil {
			return nil, []error{fmt.Errorf("read %s: %v", u.File, err)}
		}
		p := parser.New(string(data))
		f, perr := p.ParseFile()
		if perr != nil {
			return nil, []error{fmt.Errorf("parse %s: %v", u.File, perr)}
		}

		if filepath.Clean(u.File) == entryAbs {
			entryDecls = append(entryDecls, f.Decls...)
		} else {
			depDecls = append(depDecls, f.Decls...)
		}
	}

	var merged ast.File
	merged.Decls = append(merged.Decls, entryDecls...)
	merged.Decls = append(merged.Decls, depDecls...)
	return &merged, nil
}
