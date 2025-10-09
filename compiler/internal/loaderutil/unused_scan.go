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
	ImportFrom                    // from foo.bar import A, B  OR  from foo.bar import (A, B, ...)
)

// FromItem represents a single item in `from X import A, B`.
type FromItem struct {
	Name       string
	Start, End int // optional byte offsets
}

// ImportDecl is a normalized import declaration in a single file.
type ImportDecl struct {
	Kind       ImportKind
	Module     string // dotted path e.g., "util.math"
	Alias      string // for `import X as alias` (empty if none)
	Items      []FromItem
	Start, End int // optional byte offsets in source for the whole statement
}

// ScanUnusedAndDuplicateImports ...
func ScanUnusedAndDuplicateImports(src string, imps []ImportDecl) []diag.Diagnostic {
	var out []diag.Diagnostic

	// Clean text for usage scanning (don’t count comments or the import statements themselves).
	clean := stripComments(src)
	clean = stripImportStatements(clean) // now removes both single-line and parenthesized multi-line forms
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

	// duplicates within a single from-import
	seen := make(map[string]int)
	for _, it := range imp.Items {
		if _, ok := seen[it.Name]; ok {
			d := lookup("module", "duplicate_from_item", "DMW0006", "duplicate item in from-import")
			withSpan(&d, it.Start, it.End)
			d.Notes = append(d.Notes, `item "`+it.Name+`" listed more than once`)
			out = append(out, d)
		} else {
			seen[it.Name] = 1
		}
	}

	// unused
	var unused []FromItem
	for _, it := range imp.Items {
		if !wordPresent(clean, it.Name) {
			unused = append(unused, it)
		}
	}

	if len(unused) == len(imp.Items) && len(imp.Items) > 0 {
		d := lookup("module", "unused_import", "DMW0004", "unused import")
		withSpan(&d, imp.Start, imp.End)
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

func dottedPrefixPresent(clean, head string) bool {
	pat := `(?m)(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(head) + `\.`
	return regexp.MustCompile(pat).FindStringIndex(clean) != nil
}

func wordPresent(clean, ident string) bool {
	if ident == "" {
		return false
	}
	pat := `(?m)(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(ident) + `([^A-Za-z0-9_]|$)`
	return regexp.MustCompile(pat).FindStringIndex(clean) != nil
}

func stripComments(s string) string {
	s = regexp.MustCompile(`//[^\n]*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(s, "")
	return s
}

func stripImportStatements(s string) string {
	// Remove single-line imports
	reOneLine := regexp.MustCompile(`(?m)^\s*(?:import\s+[A-Za-z_][A-Za-z0-9_.]*(?:\s+as\s+[A-Za-z_][A-Za-z0-9_]*)?|from\s+[A-Za-z_][A-Za-z0-9_.]*\s+import\s+[A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)\s*$`)
	s = reOneLine.ReplaceAllString(s, "")

	// Remove parenthesized multi-line from-import blocks:
	//   from pkg.mod import (
	//       a,
	//       b,
	//       c,
	//   )
	reMultiline := regexp.MustCompile(`(?ms)^\s*from\s+[A-Za-z_][A-Za-z0-9_.]*\s+import\s*\(\s*.*?\s*\)\s*$`)
	s = reMultiline.ReplaceAllString(s, "")
	return s
}

func squashWhitespace(s string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

// lookup maps catalog → Diagnostic fields (Message, Code, Key).
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

func withSpan(_ *diag.Diagnostic, _start, _end int) {
	// no-op (can be wired to diag.Span later if desired)
}

// ----- Regex import extraction (now supports (...) multi-line) -----
var (
	reImportPlain = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_][A-Za-z0-9_.]*)(?:\s+as\s+([A-Za-z_][A-Za-z0-9_]*))?\s*$`)

	// Match either: from X import a, b
	// or:           from X import (a, b, c) with newlines/whitespace
	reImportFromParen = regexp.MustCompile(`(?ms)^\s*from\s+([A-Za-z_][A-Za-z0-9_.]*)\s+import\s*\(\s*([A-Za-z0-9_,\s]+?)\s*\)\s*$`)
	reImportFromFlat  = regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z_][A-Za-z0-9_.]*)\s+import\s+([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)\s*$`)
)

func ExtractImportsFromSource(src string) []ImportDecl {
	var out []ImportDecl

	// import X [as y]
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

	// from X import ( ... )
	for _, m := range reImportFromParen.FindAllStringSubmatchIndex(src, -1) {
		mod := src[m[2]:m[3]]
		itemsRaw := src[m[4]:m[5]]
		items := splitItems(itemsRaw)
		out = append(out, ImportDecl{
			Kind:   ImportFrom,
			Module: mod,
			Items:  items,
			Start:  m[0],
			End:    m[1],
		})
	}

	// from X import a, b (single-line)
	for _, m := range reImportFromFlat.FindAllStringSubmatchIndex(src, -1) {
		mod := src[m[2]:m[3]]
		itemsRaw := src[m[4]:m[5]]
		items := splitItems(itemsRaw)
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

func splitItems(itemsRaw string) []FromItem {
	parts := strings.Split(itemsRaw, ",")
	items := make([]FromItem, 0, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		items = append(items, FromItem{Name: name})
	}
	return items
}
