package check

import (
  "fmt"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
)

/*
Top-level pass for Phase B:

• Records visibility bits for funcs/structs into Info.{FuncsPublic,StructsPublic}
• Records top-level consts into Info.Consts (+ Info.ConstsPublic)
• Enforces:
    - DTE0012: `pub let mut` is forbidden
    - DTE0011: `pub let` must be a compile-time literal (Phase B: int/str/bool only)
    - Forbid reserved/builtins as top-level names
    - NEW: Forbid redefining names introduced by imports (aliases or from-items)
*/

func (c *checker) collectVisibilityAndConsts(f *ast.File) {
  // Ensure maps exist
  if c.info.FuncsPublic == nil {
    c.info.FuncsPublic = map[string]bool{}
  }
  if c.info.StructsPublic == nil {
    c.info.StructsPublic = map[string]bool{}
  }
  if c.info.Consts == nil {
    c.info.Consts = map[string]ConstInfo{}
  }
  if c.info.ConstsPublic == nil {
    c.info.ConstsPublic = map[string]bool{}
  }
  if c.info.ImportedNames == nil {
    c.info.ImportedNames = map[string]bool{}
  }

  // 1) Collect identifiers introduced by imports in THIS file.
  collectImportedNamesInto(c.info.ImportedNames, f)

  // 2) Process top-level declarations.
  for _, d := range f.Decls {
    switch v := d.(type) {
    case *ast.FuncDecl:
      // Forbid reserved/builtins and imported names
      if isReservedIdent(v.Name) || isPreludeBuiltin(v.Name) {
        c.errors = append(c.errors, fmt.Errorf("invalid function name %q: reserved/builtin", v.Name))
      }
      if c.info.ImportedNames[v.Name] {
        c.errors = append(c.errors, fmt.Errorf("invalid function name %q: name conflicts with an imported symbol", v.Name))
      }
      c.info.FuncsPublic[v.Name] = v.Pub

    case *ast.StructDecl:
      if isReservedIdent(v.Name) || isPreludeBuiltin(v.Name) {
        c.errors = append(c.errors, fmt.Errorf("invalid struct name %q: reserved/builtin", v.Name))
      }
      if c.info.ImportedNames[v.Name] {
        c.errors = append(c.errors, fmt.Errorf("invalid struct name %q: name conflicts with an imported symbol", v.Name))
      }
      c.info.StructsPublic[v.Name] = v.Pub

    case *ast.ConstDecl:
      // Forbid reserved/builtins and imported names
      if isReservedIdent(v.Name) || isPreludeBuiltin(v.Name) {
        c.errors = append(c.errors, fmt.Errorf("invalid constant name %q: reserved/builtin", v.Name))
      }
      if c.info.ImportedNames[v.Name] {
        c.errors = append(c.errors, fmt.Errorf("invalid constant name %q: name conflicts with an imported symbol", v.Name))
      }

      // 1) `pub let mut` is forbidden (DTE0012).
      if v.Pub && v.Mutable {
        c.errors = append(c.errors, ErrPubLetMutForbidden(v.Span, v.Name))
      }

      // 2) `pub let` must be a compile-time constant (literal only).
      ck, isConst := constKindIfLiteral(v.Value)
      if v.Pub && !isConst {
        c.errors = append(c.errors, ErrPublicConstNotConst(v.Span, v.Name))
      }

      // Record const for intra-module resolution regardless (helps local typing).
      // If not a literal, ck will be KindUnknown — fine for local typing fall-throughs.
      c.info.Consts[v.Name] = ConstInfo{Kind: ck}
      c.info.ConstsPublic[v.Name] = v.Pub && isConst && !v.Mutable
    }
  }
}

// Phase B: a constant is compile-time iff it's a bare literal (int/str/bool).
func constKindIfLiteral(e ast.Expr) (Kind, bool) {
  switch e.(type) {
  case *ast.IntLit:
    return KindInt, true
  case *ast.StrLit:
    return KindStr, true
  case *ast.BoolLit:
    return KindBool, true
  default:
    return KindUnknown, false
  }
}

/* ---------- import name collection ---------- */

// collectImportedNamesInto inspects f.Imports and f.FromImports and records the
// identifiers that become visible in the file's namespace.
func collectImportedNamesInto(dst map[string]bool, f *ast.File) {
  // Plain imports:
  //   import foo[.bar][.baz] [as alias]
  // We record the alias if present; otherwise, we record the FIRST path segment
  // (Pythonic behavior), which is consistent with earlier examples using module aliases.
  for _, imp := range f.Imports {
    path := strings.TrimSpace(imp.Path)
    if path == "" {
      continue
    }
    if alias := strings.TrimSpace(imp.As); alias != "" {
      dst[alias] = true
      continue
    }
    first := path
    if dot := strings.IndexByte(path, '.'); dot >= 0 {
      first = path[:dot]
    }
    if first != "" {
      dst[first] = true
    }
  }

  // From-imports:
  //   from mod.path import a [as x], b, c [as y]
  for _, fi := range f.FromImports {
    for _, it := range fi.Items {
      name := strings.TrimSpace(it.Name)
      if name == "" {
        continue
      }
      if alias := strings.TrimSpace(it.As); alias != "" {
        dst[alias] = true
      } else {
        dst[name] = true
      }
    }
  }
}
