package build

import (
  "fmt"
  "strings"

  "github.com/desilang/desi/compiler/internal/diag"
)

// --- tiny registry helpers (mirrors check/diagshim) ---

func lookupIDTitle(domain, key, fallbackID, fallbackTitle string) (string, string) {
  if info, ok := diag.LookupFull(domain, key); ok {
    id := info.Entry.ID
    if id == "" {
      id = fallbackID
    }
    title := info.Entry.Title
    if title == "" {
      title = fallbackTitle
    }
    return id, title
  }
  return fallbackID, fallbackTitle
}

func modMk(level diag.Level, key, fallbackID, fallbackTitle, msg string) diag.Diagnostic {
  id, _ := lookupIDTitle("module", key, fallbackID, fallbackTitle)
  return diag.Diagnostic{
    Domain:  "module",
    Key:     key,
    Level:   level,
    Code:    id,
    Message: msg,
  }
}

// ModuleErrorf: generic module-domain error with registry key + fallback.
func ModuleErrorf(key, fallbackID, fallbackTitle, format string, args ...any) error {
  msg := fmt.Sprintf(format, args...)
  return modMk(diag.LevelError, key, fallbackID, fallbackTitle, msg)
}

// --- Specific constructors used by resolver ---

// ErrImportCycle DME0001
func ErrImportCycle(chain []string) error {
  // nice “a -> b -> c -> a”
  var cyc string
  if len(chain) > 0 {
    cyc = strings.Join(append(append([]string{}, chain...), chain[0]), " -> ")
  }
  return ModuleErrorf("import_cycle", "DME0001", "import cycle",
    "import cycle detected: %s", cyc)
}

// ErrModuleNotFound DME0002
// attempts = candidate filesystem paths we looked for (repo-relative or abs).
func ErrModuleNotFound(modPath, fromFile string, attempts []string) error {
  msg := fmt.Sprintf("cannot find module %q (imported from %s)", modPath, fromFile)
  d := modMk(diag.LevelError, "not_found", "DME0002", "module not found", msg)
  // attach “looked for:” lines as notes (renderer prints these after the header)
  if len(attempts) > 0 {
    d.Notes = append(d.Notes, "looked for:")
    for _, a := range attempts {
      d.Notes = append(d.Notes, "  - "+a)
    }
  }
  return d
}

// ErrBadImport DME0003 (malformed path, etc.)
func ErrBadImport(importText, why string) error {
  return ModuleErrorf("bad_import", "DME0003", "invalid import path",
    "invalid import %q: %s", importText, why)
}

// ErrIORead DME0004 (file read failure during resolution)
func ErrIORead(path string, cause error) error {
  return ModuleErrorf("io_read", "DME0004", "failed to read file",
    "read %s: %v", path, cause)
}
