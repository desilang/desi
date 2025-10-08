package loaderutil

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// ScanDuplicateImports inspects a parsed file and returns module-domain warnings
// (DMW0001) for:
//   - duplicate plain import of the same module path
//   - duplicate from-import of the same module path
//   - duplicate imported local names within a single from-import
//
// It preserves spans when present so a renderer can show snippets.
func ScanDuplicateImports(f *ast.File) []error {
	var out []error

	// plain imports: by path
	seenPlain := map[string]struct{}{}
	for _, im := range f.Imports {
		path := strings.TrimSpace(im.Path)
		if path == "" {
			continue
		}
		if _, dup := seenPlain[path]; dup {
			out = append(out, warnDuplicateImport(
				fmt.Sprintf("module %q", path),
				spanPtrIfSet(im.Span),
			))
			continue
		}
		seenPlain[path] = struct{}{}
	}

	// from-imports: by module path
	seenFrom := map[string]struct{}{}
	for _, fm := range f.FromImports {
		mod := strings.TrimSpace(fm.Module)
		if mod != "" {
			if _, dup := seenFrom[mod]; dup {
				out = append(out, warnDuplicateImport(
					fmt.Sprintf("module %q (from-import)", mod),
					spanPtrIfSet(fm.Span),
				))
			} else {
				seenFrom[mod] = struct{}{}
			}
		}

		// duplicates within a single from-import by *local* name
		seenLocal := map[string]struct{}{}
		for _, it := range fm.Items {
			local := strings.TrimSpace(it.As)
			if local == "" {
				local = strings.TrimSpace(it.Name)
			}
			if local == "" {
				continue
			}
			if _, dup := seenLocal[local]; dup {
				out = append(out, warnDuplicateImport(
					fmt.Sprintf("symbol %q in from %q", local, mod),
					spanPtrIfSet(it.Span),
				))
				continue
			}
			seenLocal[local] = struct{}{}
		}
	}

	return out
}

func spanPtrIfSet(sp ast.Span) *ast.Span {
	if (sp.Start.Line | sp.Start.Col | sp.End.Line | sp.End.Col) == 0 {
		return nil
	}
	return &sp
}

func warnDuplicateImport(msg string, sp *ast.Span) error {
	id, title := lookupIDTitle("module", "duplicate_import", "DMW0001", "duplicate import")
	d := diag.Diagnostic{
		Domain:  "module",
		Key:     "duplicate_import",
		Level:   diag.LevelWarning,
		Code:    id,
		Message: fmt.Sprintf("%s: %s", title, msg),
	}
	if sp != nil {
		d.Span = convSpan(*sp)
	}
	return d
}

// --- tiny local diag helpers (kept private to avoid importing build/diagshim) ---

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

func convSpan(sp ast.Span) diag.Span {
	return diag.Span{
		Start: diag.Pos{Line: sp.Start.Line, Col: sp.Start.Col},
		End:   diag.Pos{Line: sp.End.Line, Col: sp.End.Col},
	}
}
