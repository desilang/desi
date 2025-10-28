package resolve

import (
  "fmt"
  "os"
  "path/filepath"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/parse"
)

// FSLoader maps dotted module names -> files under a single root.
// Package rules (Phase-1):
//   - Each intermediate segment MUST be a package directory containing __mod.desi
//   - The leaf may be either `<dir>/<leaf>/__mod.desi` (subpackage) OR `<dir>/<leaf>.desi` (leaf file module)
type FSLoader struct {
  Root string
}

func NewFSLoader(root string) *FSLoader { return &FSLoader{Root: root} }

// dotted "a.b.c" -> (<root>/a/__mod.desi) + (either <root>/a/b/__mod.desi) + (either <root>/a/b/c/__mod.desi OR <root>/a/b/c.desi)
func (l *FSLoader) Load(dotted string) (*ast.Module, []error, error) {
  if l == nil || l.Root == "" {
    return nil, nil, fmt.Errorf("fs loader has empty root")
  }

  parts := strings.Split(dotted, ".")
  // Walk prefix packages: a, a/b, ...
  cur := l.Root

  for i := 0; i < len(parts)-1; i++ {
    cur = filepath.Join(cur, parts[i])
    mod := filepath.Join(cur, "__mod.desi")
    if st, err := os.Stat(mod); err != nil || st.IsDir() {
      return nil, nil, fmt.Errorf("package not found: %s (missing __mod.desi)", strings.Join(parts[:i+1], "."))
    }
    // We do NOT parse intermediates here; Resolve() will Load() each module as needed.
  }

  // Leaf resolution: prefer subpackage leaf, else leaf file module.
  leafDir := filepath.Join(cur, parts[len(parts)-1])
  leafPkg := filepath.Join(leafDir, "__mod.desi")
  leafFile := filepath.Join(cur, parts[len(parts)-1]+".desi")

  var path string
  if st, err := os.Stat(leafPkg); err == nil && !st.IsDir() {
    path = leafPkg
  } else if st, err := os.Stat(leafFile); err == nil && !st.IsDir() {
    path = leafFile
  } else {
    return nil, nil, fmt.Errorf("module not found: %s", dotted)
  }

  src, err := os.ReadFile(path)
  if err != nil {
    return nil, nil, err
  }
  mod := parse.ParseFile(path, src)
  // FSLoader returns structural parse errors in mod.Diags; bubble them up as []error for Resolve to re-map.
  var errs []error
  for _, d := range mod.Diags {
    errs = append(errs, fmt.Errorf("%s", d.Message))
  }
  return mod, errs, nil
}
