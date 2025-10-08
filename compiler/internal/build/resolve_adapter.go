package build

import (
	"os"
	"path/filepath"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/loaderutil"
	"github.com/desilang/desi/compiler/internal/parser"
)

/*
Adapters that keep the CLI stable.

We implement the old function shapes that the CLI calls today:

  ResolveAndParseWithParserBridge(rootDir string, useBridge bool, entryPath string, allowStd bool, stage1 bool)
  ResolveAndParseMaybeDesi(entryPath string, allowStd bool, _stage0 bool, _stage1 bool)

The extra flags are ignored by the unified resolver. We still honor `allowStd`
on the ResolveOptions for future use.
*/

// Common worker that calls the unified resolver and merges ASTs (entry first).
func runResolveAndParse(entryPath string, opts ResolveOptions) (*ast.File, []error) {
	plan, mdiags, rerr := ResolveEntry(entryPath, opts)
	if rerr != nil {
		return nil, []error{ErrInternalf("resolve: %v", rerr)}
	}
	if len(mdiags) > 0 {
		errs := make([]error, 0, len(mdiags))
		for _, d := range mdiags {
			// Forward module-domain diagnostics (they implement error).
			errs = append(errs, d)
		}
		return nil, errs
	}

	entryAbs := filepath.Clean(plan.Entry.File)
	var (
		entryDecls []ast.Decl
		depDecls   []ast.Decl
		diags      []error // warnings (e.g., duplicate/self/alias conflicts)
	)

	for _, u := range plan.Deps {
		data, err := os.ReadFile(u.File)
		if err != nil {
			return nil, []error{ErrIORead(u.File, err)}
		}
		p := parser.New(string(data))
		f, perr := p.ParseFile()
		if perr != nil {
			// Parser errors are returned as plain errors; the caller renders them.
			return nil, []error{ErrParseFailed(u.File, perr)}
		}

		// collect module-domain warnings for this file
		diags = append(diags, loaderutil.ScanDuplicateImports(f)...)
		diags = append(diags, loaderutil.ScanSelfImport(f, u.Module)...)
		diags = append(diags, loaderutil.ScanImportAliasConflicts(f)...)

		if filepath.Clean(u.File) == entryAbs {
			entryDecls = append(entryDecls, f.Decls...)
		} else {
			depDecls = append(depDecls, f.Decls...)
		}
	}

	var merged ast.File
	merged.Decls = append(merged.Decls, entryDecls...)
	merged.Decls = append(merged.Decls, depDecls...)
	return &merged, diags
}

// ResolveAndParseWithParserBridge kept for CLI compatibility.
// Old signature: (rootDir string, useBridge bool, entryPath string, allowStd bool, stage1 bool)
func ResolveAndParseWithParserBridge(_rootDir string, _useBridge bool, entryPath string, allowStd bool, _stage1 bool) (*ast.File, []error) {
	return runResolveAndParse(entryPath, ResolveOptions{AllowStd: allowStd})
}

// ResolveAndParseMaybeDesi kept for CLI compatibility.
// Old signature: (entryPath string, allowStd bool, _stage0 bool, _stage1 bool)
func ResolveAndParseMaybeDesi(entryPath string, allowStd bool, _stage0 bool, _stage1 bool) (*ast.File, []error) {
	return runResolveAndParse(entryPath, ResolveOptions{AllowStd: allowStd})
}

// Direct resolver with diagnostics (useful for tooling/tests).
func ResolveEntryWithDiagnostics(entryPath string) (Plan, []diag.Diagnostic, error) {
	return ResolveEntry(entryPath, ResolveOptions{})
}
