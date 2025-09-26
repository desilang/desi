package build

import (
  "fmt"
  "os"
  "path/filepath"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/lexbridge"
  "github.com/desilang/desi/compiler/internal/parsebridge"
  "github.com/desilang/desi/compiler/internal/parser"
)

/* ---------- typed module diagnostics (lightweight) ---------- */

type modErr struct {
  code   string // DME0001/2 (reserved for errors)
  title  string
  key    string // stable key in catalog, e.g. "import_cycle"
  detail string // extra context shown after the title line
}

func (e modErr) Error() string {
  if strings.TrimSpace(e.detail) == "" {
    return e.title
  }
  return e.title + ": " + e.detail
}
func (e modErr) Code() string   { return e.code }
func (e modErr) Title() string  { return e.title }
func (e modErr) Domain() string { return "module" }
func (e modErr) Key() string    { return e.key }

/* ---------- utilities ---------- */

func fileExists(p string) bool {
  _, err := os.Stat(p)
  return err == nil
}
func dirExists(p string) bool {
  fi, err := os.Stat(p)
  return err == nil && fi.IsDir()
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

// collectLibRoots walks upward from startDir to / and returns every ancestor "compiler/lib" dir.
// This avoids false-positives like "examples/compiler" and is future-proof if users add their own compiler/.
func collectLibRoots(startDir string) []string {
  var libs []string
  seen := map[string]bool{}
  cur := startDir
  for {
    cand := filepath.Join(cur, "compiler", "lib")
    if dirExists(cand) && !seen[cand] {
      libs = append(libs, cand)
      seen[cand] = true
    }
    parent := filepath.Dir(cur)
    if parent == cur {
      break
    }
    cur = parent
  }
  return libs
}

// buildSearchRoots returns [projectDir, <each ancestor compiler/lib>...] in order.
func buildSearchRoots(entryAbs string) []string {
  projectDir := filepath.Dir(entryAbs)
  roots := []string{projectDir}
  roots = append(roots, collectLibRoots(projectDir)...)
  return roots
}

// resolveModule tries X.Y.Z.desi across roots. Returns first hit or "".
func resolveModule(roots []string, dotted string) (string, []string) {
  relPath := strings.ReplaceAll(dotted, ".", string(filepath.Separator)) + ".desi"
  tried := make([]string, 0, len(roots))
  for _, r := range roots {
    cand := filepath.Join(r, relPath)
    tried = append(tried, cand)
    if fileExists(cand) {
      return cand, tried
    }
  }
  return "", tried
}

// formatChain a → b → c
func formatChain(chain []string, root string) string {
  if len(chain) == 0 {
    return ""
  }
  var parts []string
  for _, p := range chain {
    parts = append(parts, rel(root, p))
  }
  return strings.Join(parts, " \u2192 ")
}

/* ---------- Stage-0 (Go lexer) path ---------- */

func ResolveAndParse(entryPath string) (*ast.File, []error) {
  entryAbs, err := filepath.Abs(entryPath)
  if err != nil {
    return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
  }
  rootDir := filepath.Dir(entryAbs)
  roots := buildSearchRoots(entryAbs)

  type unit struct {
    path string // absolute file path
    file *ast.File
  }
  var (
    errs   []error
    seen   = map[string]bool{} // absolute path → true
    stack  = []string{}        // for cycle diagnostics (abs paths)
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
        chain := append(append([]string{}, stack...), absPath)
        errs = append(errs, modErr{
          code:   "DME0001",
          title:  "import cycle",
          key:    "import_cycle",
          detail: formatChain(chain, rootDir),
        })
        return
      }
    }
    stack = append(stack, absPath)
    defer func() { stack = stack[:len(stack)-1] }()

    data, rerr := os.ReadFile(absPath)
    if rerr != nil {
      errs = append(errs, fmt.Errorf("read %s: %v", rel(rootDir, absPath), rerr))
      return
    }
    p := parser.New(string(data))
    f, perr := p.ParseFile()
    if perr != nil {
      errs = append(errs, fmt.Errorf("parse %s: %v", rel(rootDir, absPath), perr))
      return
    }

    // resolve imports (std.* resolves via any ancestor compiler/lib)
    for _, imp := range f.Imports {
      path := strings.TrimSpace(imp.Path)
      if path == "" {
        continue
      }
      target, tried := resolveModule(roots, path)
      if target == "" {
        errs = append(errs, modErr{
          code:  "DME0002",
          title: "cannot find module",
          key:   "missing_module",
          detail: fmt.Sprintf("%q (looked for: %s)", path, strings.Join(func(ss []string) []string {
            out := make([]string, len(ss))
            for i, s := range ss {
              out[i] = rel(rootDir, s)
            }
            return out
          }(tried), ", ")),
        })
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

  // Merge: entry first, then others.
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

/* ---------- Stage-1 (Desi lexer via bridge, Go parser) ---------- */

func ResolveAndParseWith(entryPath string, loader func(absPath string) (parser.TokenSource, error)) (*ast.File, []error) {
  entryAbs, err := filepath.Abs(entryPath)
  if err != nil {
    return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
  }
  rootDir := filepath.Dir(entryAbs)
  roots := buildSearchRoots(entryAbs)

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
    for _, on := range stack {
      if on == absPath {
        chain := append(append([]string{}, stack...), absPath)
        errs = append(errs, modErr{
          code:   "DME0001",
          title:  "import cycle",
          key:    "import_cycle",
          detail: formatChain(chain, rootDir),
        })
        return
      }
    }
    stack = append(stack, absPath)
    defer func() { stack = stack[:len(stack)-1] }()

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

    for _, imp := range f.Imports {
      path := strings.TrimSpace(imp.Path)
      if path == "" {
        continue
      }
      target, tried := resolveModule(roots, path)
      if target == "" {
        errs = append(errs, modErr{
          code:  "DME0002",
          title: "cannot find module",
          key:   "missing_module",
          detail: fmt.Sprintf("%q (looked for: %s)", path, strings.Join(func(ss []string) []string {
            out := make([]string, len(ss))
            for i, s := range ss {
              out[i] = rel(rootDir, s)
            }
            return out
          }(tried), ", ")),
        })
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

/* ---------- Stage-1 (Desi parser bridge) ---------- */

func ResolveAndParseMaybeDesi(entryPath string, useDesiLexer bool, keepTmp, verbose bool) (*ast.File, []error) {
  if !useDesiLexer {
    return ResolveAndParse(entryPath)
  }
  loader := func(absPath string) (parser.TokenSource, error) {
    return lexbridge.NewSourceFromFileOpts(absPath, keepTmp, verbose)
  }
  return ResolveAndParseWith(entryPath, loader)
}

func ResolveAndParseWithParserBridge(entryFile string, useExternal bool, bin string, keepTmp, verbose bool) (*ast.File, []error) {
  entryAbs, err := filepath.Abs(entryFile)
  if err != nil {
    return nil, []error{fmt.Errorf("abs(%s): %v", entryFile, err)}
  }
  rootDir := filepath.Dir(entryAbs)
  roots := buildSearchRoots(entryAbs)

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

  var parseOne func(absPath string) *ast.File
  parseOne = func(absPath string) *ast.File {
    var js []byte
    var err error
    if useExternal {
      js, err = parsebridge.Run(absPath, bin, verbose)
    } else {
      js, err = parsebridge.BuildAndRunJSON(absPath, keepTmp, verbose)
    }
    if err != nil {
      errs = append(errs, fmt.Errorf("bridge parse %s: %v", rel(rootDir, absPath), err))
      return nil
    }
    f, uerr := ast.UnmarshalFileJSON(js)
    if uerr != nil {
      errs = append(errs, fmt.Errorf("bridge AST JSON invalid for %s: %v", rel(rootDir, absPath), uerr))
      return nil
    }
    return f
  }

  var load func(absPath string)
  load = func(absPath string) {
    if seen[absPath] {
      return
    }
    for _, on := range stack {
      if on == absPath {
        chain := append(append([]string{}, stack...), absPath)
        errs = append(errs, modErr{
          code:   "DME0001",
          title:  "import cycle",
          key:    "import_cycle",
          detail: formatChain(chain, rootDir),
        })
        return
      }
    }
    stack = append(stack, absPath)
    defer func() { stack = stack[:len(stack)-1] }()

    if !fileExists(absPath) {
      errs = append(errs, fmt.Errorf("read %s: not found", rel(rootDir, absPath)))
      return
    }

    f := parseOne(absPath)
    if f == nil {
      return
    }

    for _, im := range f.Imports {
      path := strings.TrimSpace(im.Path)
      if path == "" {
        continue
      }
      target, tried := resolveModule(roots, path)
      if target == "" {
        errs = append(errs, modErr{
          code:  "DME0002",
          title: "cannot find module",
          key:   "missing_module",
          detail: fmt.Sprintf("%q (looked for: %s)", path, strings.Join(func(ss []string) []string {
            out := make([]string, len(ss))
            for i, s := range ss {
              out[i] = rel(rootDir, s)
            }
            return out
          }(tried), ", ")),
        })
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
