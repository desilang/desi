package build

import (
  "os"
  "path/filepath"
  "sort"
  "strings"

  "github.com/desilang/desi/compiler/internal/diag"
)

// ResolveOptions controls search roots.
type ResolveOptions struct {
  // AllowStd reserved for future (on by default).
  AllowStd bool
  // Roots adds additional search roots ahead of auto-detected ones.
  Roots []string
}

// Unit identifies a compilation unit by dotted module name and absolute file path.
type Unit struct {
  Module string
  File   string
}

// Plan is a topologically ordered list of units: Entry first, then its deps.
type Plan struct {
  Entry Unit
  Deps  []Unit // includes Entry as the first element
}

// ResolveEntry constructs the dependency plan for the given entry file.
// Returns the plan, any user-facing diagnostics (module domain), and a fatal error
// only for system failures (permissions, I/O), not user mistakes.
func ResolveEntry(entryPath string, opts ResolveOptions) (Plan, []diag.Diagnostic, error) {
  var plan Plan
  var diags []diag.Diagnostic

  if entryPath == "" {
    return plan, diags, ModuleErrorf("bad_entry", "DME0006", "bad entry path",
      "resolve: entry path is empty")
  }
  entryAbs, err := filepath.Abs(entryPath)
  if err != nil {
    return plan, diags, ErrBadEntryPath(entryPath, err)
  }
  fi, err := os.Stat(entryAbs)
  if err != nil || fi.IsDir() {
    return plan, diags, ErrEntryNotFound(entryAbs)
  }

  roots := uniqStrings(append([]string{}, opts.Roots...))
  auto := autoRoots(filepath.Dir(entryAbs))
  roots = append(roots, auto...)
  roots = filterExistingDirs(roots)
  if len(roots) == 0 {
    // Fallback: at least use the entry directory.
    roots = []string{filepath.Dir(entryAbs)}
  }

  // Build import graph starting at entryAbs
  g := newGraph(roots)
  entryMod := g.moduleNameForFile(entryAbs)
  if entryMod == "" {
    // If not under a root, treat as a single-file module by basename.
    entryMod = strings.TrimSuffix(filepath.Base(entryAbs), filepath.Ext(entryAbs))
  }
  g.addFile(entryAbs, entryMod)
  // Walk closure
  g.walk(entryAbs)

  // Collect diagnostics: not founds (already recorded) + cycles
  diags = append(diags, g.diags...)

  // If cycle detected, return early with partial plan (just entry).
  if len(g.cycles) > 0 {
    plan = Plan{Entry: Unit{Module: entryMod, File: entryAbs}, Deps: []Unit{{Module: entryMod, File: entryAbs}}}
    return plan, diags, nil
  }

  // Topo order
  order := g.topo()
  // First in order should be entryAbs; ensure it is, else prepend.
  var deps []Unit
  seen := map[string]bool{}
  for _, f := range order {
    m := g.fileToModule[f]
    if m == "" {
      m = g.moduleNameForFile(f)
    }
    if !seen[f] {
      deps = append(deps, Unit{Module: m, File: f})
      seen[f] = true
    }
  }
  if len(deps) == 0 || filepath.Clean(deps[0].File) != filepath.Clean(entryAbs) {
    deps = append([]Unit{{Module: entryMod, File: entryAbs}}, deps...)
  }
  plan = Plan{Entry: deps[0], Deps: deps}
  return plan, diags, nil
}

func uniqStrings(in []string) []string {
  m := map[string]struct{}{}
  var out []string
  for _, s := range in {
    s = filepath.Clean(s)
    if _, ok := m[s]; !ok {
      m[s] = struct{}{}
      out = append(out, s)
    }
  }
  return out
}

func filterExistingDirs(in []string) []string {
  var out []string
  for _, d := range in {
    if st, err := os.Stat(d); err == nil && st.IsDir() {
      out = append(out, d)
    }
  }
  return out
}

// autoRoots returns: [entryDir, each ancestor with compiler/lib, each ancestor itself]
func autoRoots(entryDir string) []string {
  var roots []string
  // Always include the entry directory as a root.
  roots = append(roots, entryDir)

  // Walk up to filesystem root; add "<ancestor>/compiler/lib" if it exists,
  // and also add the ancestor itself (to allow project-local modules).
  dir := entryDir
  for i := 0; i < 12; i++ { // reasonable guard
    if dir == "" || dir == "/" || dir == "." {
      break
    }
    cl := filepath.Join(dir, "compiler", "lib")
    if st, err := os.Stat(cl); err == nil && st.IsDir() {
      roots = append(roots, cl)
    }
    roots = append(roots, dir)
    next := filepath.Dir(dir)
    if next == dir {
      break
    }
    dir = next
  }
  // stabilize order
  roots = uniqStrings(roots)
  sort.Strings(roots)
  return roots
}
