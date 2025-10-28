package resolve

import (
  "fmt"
  "os"
  "path/filepath"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/diag"
  "github.com/desilang/desi/compiler/internal/parse"
)

// FSLoader resolves dotted module names against a list of search roots.
// Example roots: project src root, and compiler/lib (stdlib).
type FSLoader struct {
  Roots []string // searched in order; first match wins
}

func NewFSLoader(roots ...string) *FSLoader {
  return &FSLoader{Roots: append([]string(nil), roots...)}
}

func (l *FSLoader) Load(dotted string) (*ast.Module, []diag.Diagnostic, error) {
  parts := splitDotted(dotted)
  if len(parts) == 0 {
    return nil, nil, fmt.Errorf("empty module path")
  }
  for _, root := range l.Roots {
    if mod, diags, ok := l.tryRoot(root, parts); ok {
      return mod, diags, nil
    }
  }
  return nil, nil, fmt.Errorf("module not found: %s", dotted)
}

func (l *FSLoader) tryRoot(root string, parts []string) (*ast.Module, []diag.Diagnostic, bool) {
  // Validate all intermediate packages under this root: a/__mod.desi, a/b/__mod.desi, …
  cur := filepath.Clean(root)
  for i := 0; i < len(parts)-1; i++ {
    cur = filepath.Join(cur, parts[i])
    pkgInit := filepath.Join(cur, "__mod.desi")
    if !isFile(pkgInit) {
      return nil, nil, false
    }
  }
  // Final segment: prefer package initializer, else leaf file module.
  cur = filepath.Join(cur, parts[len(parts)-1])
  if isFile(filepath.Join(cur, "__mod.desi")) {
    return l.parseFile(filepath.Join(cur, "__mod.desi"))
  }
  if isFile(cur + ".desi") {
    return l.parseFile(cur + ".desi")
  }
  return nil, nil, false
}

func (l *FSLoader) parseFile(path string) (*ast.Module, []diag.Diagnostic, bool) {
  data, err := os.ReadFile(path)
  if err != nil {
    return nil, nil, false
  }
  mod, diags := parse.ParseFile(path, data)
  return mod, diags, true
}

func isFile(p string) bool {
  st, err := os.Stat(p)
  return err == nil && !st.IsDir()
}
