package build

import (
  "os"
  "path/filepath"
  "strings"
)

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

// resolveModule tries X.Y.Z.desi across roots. Returns first hit or "" (with all tried paths).
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
