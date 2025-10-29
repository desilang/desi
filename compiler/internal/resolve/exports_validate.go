package resolve

import (
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/diag"
)

// ValidateFromItemExports verifies that every "from X import name [as local]"
// refers to an actually EXPORTED function name of module X.
// Export rule (Phase-2):
//   - functions only
//   - must be fully typed (params+return)
//   - (pub gating to be enforced once parser accepts `pub def`)
//
// It returns diagnostics (DME0003) for any missing exported items.
// NOTE: this helper does not mutate resolver Info; it's a pure check. The
// checker or CLI can append the returned diagnostics to the overall list.
func ValidateFromItemExports(mod *ast.Module, ldr Loader) []diag.Diagnostic {
  var out []diag.Diagnostic
  if mod == nil || ldr == nil {
    return out
  }
  for _, d := range mod.Decls {
    fr, ok := d.(*ast.FromImportStmt)
    if !ok {
      continue
    }
    mpath := strings.Join(fr.Path, ".")
    if mpath == "" {
      // Parser guarantees a non-empty path; guard anyway.
      continue
    }
    tmod, diags, _ := ldr.Load(mpath)
    // If loader produced parse diagnostics for the target module, surface them;
    // Resolve(...) typically aggregates these. We re-emit here for standalone use.
    if len(diags) > 0 {
      out = append(out, diags...)
    }
    if tmod == nil {
      // Loader error already covered; skip export checks for this path.
      continue
    }
    exp := CollectExports(tmod)
    for _, it := range fr.Items {
      name := it.Name.Name
      if name == "" {
        continue
      }
      if exp == nil || len(exp.Funcs[name]) == 0 {
        msg := mpath + ` has no exported '` + name + `'`
        out = append(out, diagAt("DME0003", it.Span, msg))
      }
    }
  }
  return out
}
