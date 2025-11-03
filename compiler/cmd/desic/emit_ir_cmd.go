package main

import (
  "fmt"
  "os"
  "path/filepath"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/backend/llvm"
  "github.com/desilang/desi/compiler/internal/hir"
  "github.com/desilang/desi/compiler/internal/lower"
  "github.com/desilang/desi/compiler/internal/parse"
  "github.com/desilang/desi/compiler/internal/term"
)

// init() runs before main(). We intercept "emit-ir" here and exit after handling it.
// This avoids modifying the existing main.go.
func init() {
  if len(os.Args) < 2 || os.Args[1] != "emit-ir" {
    return
  }

  file, err := parseEmitArgs(os.Args[2:])
  if err != nil {
    term.Eprintln("emit-ir error:", err)
    term.Flush()
    os.Exit(2)
  }

  // Read + parse
  src, err := os.ReadFile(file)
  if err != nil {
    term.Eprintln("emit-ir read error:", err)
    term.Flush()
    os.Exit(2)
  }
  mod, pdiags := parse.ParseFile(file, src)
  if len(pdiags) > 0 {
    // Render parser diags briefly; keep it simple here.
    // (We rely on existing diag rendering in other codepaths.)
    term.Eprintln("emit-ir: parse produced", len(pdiags), "diagnostic(s); continuing may fail")
  }

  // Find def main() with a body
  fnDecl := findMainFunc(mod)
  if fnDecl == nil || fnDecl.Body == nil {
    term.Eprintln("emit-ir:", filepath.Base(file)+": def main() not found or has no body")
    term.Flush()
    os.Exit(2)
  }

  // Lower to HIR (structural is fine for Tier-0)
  hf := lower.LowerBlock("main", fnDecl.Body)

  // Emit textual LLVM IR
  m := llvm.NewModule("main")
  m.EmitFunc(hf)
  fmt.Print(m.IR())
  term.Flush()
  os.Exit(0)
}

func parseEmitArgs(argv []string) (string, error) {
  // Very simple: one positional <file>. (We can add -I later if needed.)
  if len(argv) == 0 {
    return "", fmt.Errorf("usage: desic emit-ir <file.desi>")
  }
  // Ignore extra args for now; accept the first non-flag as the file.
  for _, a := range argv {
    if len(a) > 0 && a[0] != '-' {
      return a, nil
    }
  }
  return "", fmt.Errorf("missing <file.desi>")
}

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

// Ensure imports are not trimmed by the compiler when building this file.
// (hir is used via lower; keep an explicit reference to avoid tooling warnings.)
var _ = hir.TypeInt
