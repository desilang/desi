package parsebridge

import (
  "fmt"
  "os"
  "path/filepath"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/build"
  "github.com/desilang/desi/compiler/internal/lexer"
  "github.com/desilang/desi/compiler/internal/loaderutil"
  "github.com/desilang/desi/compiler/internal/parser"
)

// resolveAndParseLocal resolves imports (entry-first order), parses all units,
// runs lightweight import warnings (unused imports / duplicate from-items),
// and returns a merged AST plus any warnings as []error.
//
// Signature matches the legacy bridge expectations: (rootDir, entryPath string).
// The rootDir parameter is not required by the unified resolver and is ignored.
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
    // Return only module-domain diags to caller; they render at the CLI level.
    return nil, errs
  }

  entryAbs := filepath.Clean(plan.Entry.File)
  var (
    entryDecls []ast.Decl
    depDecls   []ast.Decl
    warns      []error
  )

  for _, u := range plan.Deps {
    srcBytes, rerr := os.ReadFile(u.File)
    if rerr != nil {
      return nil, []error{fmt.Errorf("read %s: %v", u.File, rerr)}
    }
    src := string(srcBytes)

    // Parse with Stage-0 lexer+parser.
    p := parser.NewFromSource(lexer.NewSource(src))
    f, perr := p.ParseFile()
    if perr != nil {
      // Keep parser errors typed; bridge caller renders them nicely.
      return nil, []error{perr}
    }

    // Import warnings on this unit (regex-based extraction for low coupling).
    imps := loaderutil.ExtractImportsFromSource(src)
    idiags := loaderutil.ScanUnusedAndDuplicateImports(src, imps)
    for _, d := range idiags {
      warns = append(warns, d)
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
  return &merged, warns
}
