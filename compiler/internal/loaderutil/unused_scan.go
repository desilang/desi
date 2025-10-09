// compiler/internal/loaderutil/unused_scan.go
package loaderutil

import (
  "regexp"
  "sort"
  "strings"

  "github.com/desilang/desi/compiler/internal/diag"
)

// ----- Public inputs (normalized import model) -----

// ImportKind classifies the import flavor.
type ImportKind int

const (
  ImportPlain ImportKind = iota // import foo.bar [as baz]
  ImportFrom                    // from foo.bar import A, B
)

// FromItem represents a single item in `from X import A, B`.
type FromItem struct {
  Name string
  // Optional byte offsets in source for precise spans (start inclusive, end exclusive).
  // Leave zero to omit spans; callers can fill if they track tokens.
  Start int
  End   int
}

// ImportDecl is a normalized import declaration in a single file.
type ImportDecl struct {
  Kind   ImportKind
  Module string // dotted path e.g., "util.math"
  Alias  string // for `import X as alias` (empty if none)
  Items  []FromItem
  // Optional byte offsets in source for whole import statement.
  Start int
  End   int
}

// ScanUnusedAndDuplicateImports analyzes a file's source and normalized imports,
// returning diagnostics for:
//   - module.unused_import (DMW0004)
//   - module.unused_from_item (DMW0005)
//   - module.duplicate_from_item (DMW0006)
func ScanUnusedAndDuplicateImports(src string, imps []ImportDecl) []diag.Diagnostic {
  var out []diag.Diagnostic

  // Pre-compute a fast searchable, comment-free text to reduce false positives.
  // IMPORTANT: also strip the import lines themselves, so identifiers present
  // only on import statements aren’t counted as “used”.
  clean := stripComments(src)
  clean = stripImportStatements(clean)
  clean = squashWhitespace(clean)

  for _, imp := range imps {
    switch imp.Kind {
    case ImportPlain:
      out = append(out, scanPlainImport(clean, imp)...)
    case ImportFrom:
      out = append(out, scanFromImport(clean, imp)...)
    }
  }

  return out
}

// ----- Implementation -----

func scanPlainImport(clean string, imp ImportDecl) []diag.Diagnostic {
  // plain: import util.math [as m]
  // usage considered "used" if either:
  //   - alias identifier appears as a whole word, or
  //   - module head appears as a qualified whole word (first segment), e.g., "util" or "util."
  alias := strings.TrimSpace(imp.Alias)
  modHead := headOf(imp.Module)

  used := false
  if alias != "" && wordPresent(clean, alias) {
    used = true
  } else if modHead != "" && (wordPresent(clean, modHead) || dottedPrefixPresent(clean, modHead)) {
    used = true
  }

  if !used {
    d := lookup("module", "unused_import", "DMW0004", "unused import")
    withSpan(&d, imp.Start, imp.End)
    // Add a helpful note with the module (and alias if present).
    if alias != "" {
      d.Notes = append(d.Notes, `imported as "`+alias+`" from `+imp.Module)
    } else {
      d.Notes = append(d.Notes, `imported module `+imp.Module)
    }
    return []diag.Diagnostic{d}
  }
  return nil
}

func scanFromImport(clean string, imp ImportDecl) []diag.Diagnostic {
  var out []diag.Diagnostic

  // 1) detect duplicate items within the same `from` statement
  seen := make(map[string]int)
  for _, it := range imp.Items {
    name := it.Name
    if prev, ok := seen[name]; ok {
      _ = prev
      d := lookup("module", "duplicate_from_item", "DMW0006", "duplicate item in from-import")
      withSpan(&d, it.Start, it.End)
      d.Notes = append(d.Notes, `item "`+name+`" listed more than once`)
      out = append(out, d)
    } else {
      seen[name] = 1
    }
  }

  // 2) detect unused items
  var unused []FromItem
  for _, it := range imp.Items {
    if !wordPresent(clean, it.Name) {
      unused = append(unused, it)
    }
  }

  if len(unused) == len(imp.Items) && len(imp.Items) > 0 {
    // All items unused → single DMW0004 on the from-import line with notes per item.
    d := lookup("module", "unused_import", "DMW0004", "unused import")
    withSpan(&d, imp.Start, imp.End)
    // Sort names for stable output
    names := make([]string, 0, len(unused))
    for _, it := range unused {
      names = append(names, it.Name)
    }
    sort.Strings(names)
    for _, n := range names {
      d.Notes = append(d.Notes, `unused item "`+n+`"`)
    }
    out = append(out, d)
  } else {
    // Some unused → itemized DMW0005 per unused item
    for _, it := range unused {
      d := lookup("module", "unused_from_item", "DMW0005", "unused imported name")
      withSpan(&d, it.Start, it.End)
      d.Notes = append(d.Notes, `remove unused item "`+it.Name+`"`)
      out = append(out, d)
    }
  }

  return out
}

func headOf(mod string) string {
  if mod == "" {
    return ""
  }
  if i := strings.IndexByte(mod, '.'); i >= 0 {
    return mod[:i]
  }
  return mod
}

func dottedPrefixPresent(clean string, head string) bool {
  // look for head followed by a dot as a word boundary, e.g., `\butil\.`
  pat := `(?m)(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(head) + `\.`
  re := regexp.MustCompile(pat)
  return re.FindStringIndex(clean) != nil
}

func wordPresent(clean string, ident string) bool {
  if ident == "" {
    return false
  }
  // whole word boundary-ish check (Desi identifiers are [A-Za-z_][A-Za-z0-9_]*)
  pat := `(?m)(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(ident) + `([^A-Za-z0-9_]|$)`
  re := regexp.MustCompile(pat)
  return re.FindStringIndex(clean) != nil
}

func stripComments(s string) string {
  // very light stripper: remove //… and /*…*/. Good enough for usage scanning.
  // This avoids counting identifiers inside comments as "used".
  s = regexp.MustCompile(`//[^\n]*`).ReplaceAllString(s, "")
  s = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(s, "")
  return s
}

func stripImportStatements(s string) string {
  // Remove entire import/from lines so that identifiers on those lines
  // don't count as "usage".
  re := regexp.MustCompile(`(?m)^\s*(?:import\s+[A-Za-z_][A-Za-z0-9_.]*(?:\s+as\s+[A-Za-z_][A-Za-z0-9_]*)?|from\s+[A-Za-z_][A-Za-z0-9_.]*\s+import\s+[A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)\s*$`)
  return re.ReplaceAllString(s, "")
}

func squashWhitespace(s string) string {
  return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

// lookup fills Key/Message/Code from the catalog when available; uses provided fallbacks otherwise.
// NOTE: Your Diagnostic has Message (not Title) and no Help field, so we map:
//   - Key   = catalog key
//   - Code  = catalog id (e.g., DMW0004)
//   - Message = catalog title (fallback to provided title)
//   - If catalog has Help, we append it as a note for convenience.
func lookup(domain, key, fallbackID, fallbackTitle string) diag.Diagnostic {
  d := diag.Diagnostic{
    Domain:  domain,
    Key:     key,
    Code:    fallbackID,
    Level:   diag.LevelWarning,
    Message: fallbackTitle,
  }
  if info, ok := diag.LookupFull(domain, key); ok {
    if id := info.Entry.ID; id != "" {
      d.Code = id
    }
    if title := info.Entry.Title; title != "" {
      d.Message = title
    }
    if help := info.Entry.Help; help != "" {
      d.Notes = append(d.Notes, help)
    }
  }
  return d
}

// withSpan attaches byte-offset spans if provided (non-zero).
// If both are zero, leaves Span nil (renderers should tolerate missing spans).
func withSpan(_ *diag.Diagnostic, _start, _end int) {
  // No-op by default; adapt if you later wire byte offsets -> diag.Span.
}

// ----- Optional helper: regex-only import extraction (for integrations without AST) -----
var (
  reImportPlain = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_][A-Za-z0-9_.]*)(?:\s+as\s+([A-Za-z_][A-Za-z0-9_]*))?\s*$`)
  reImportFrom  = regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z_][A-Za-z0-9_.]*)\s+import\s+([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)\s*$`)
)

// ExtractImportsFromSource provides a best-effort import list using regex only.
// This is intentionally simple and may be replaced by an AST-driven adapter.
func ExtractImportsFromSource(src string) []ImportDecl {
  var out []ImportDecl

  for _, m := range reImportPlain.FindAllStringSubmatchIndex(src, -1) {
    mod := src[m[2]:m[3]]
    alias := ""
    if len(m) >= 6 && m[4] >= 0 && m[5] >= 0 {
      alias = src[m[4]:m[5]]
    }
    out = append(out, ImportDecl{
      Kind:   ImportPlain,
      Module: mod,
      Alias:  alias,
      Start:  m[0],
      End:    m[1],
    })
  }

  for _, m := range reImportFrom.FindAllStringSubmatchIndex(src, -1) {
    mod := src[m[2]:m[3]]
    itemsRaw := src[m[4]:m[5]]
    parts := strings.Split(itemsRaw, ",")
    items := make([]FromItem, 0, len(parts))
    // We don't have precise per-item spans here; leave zero to omit spans.
    for _, p := range parts {
      name := strings.TrimSpace(p)
      if name == "" {
        continue
      }
      items = append(items, FromItem{Name: name})
    }
    out = append(out, ImportDecl{
      Kind:   ImportFrom,
      Module: mod,
      Items:  items,
      Start:  m[0],
      End:    m[1],
    })
  }

  return out
}
