package loaderutil

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

/* ---------- helpers ---------- */

func lookupCodeTitle(domain, key, fallbackID, fallbackTitle string) (string, string) {
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

func spanFromAST(s ast.Span) diag.Span {
	return diag.Span{
		Start: diag.Pos{Line: s.Start.Line, Col: s.Start.Col},
		End:   diag.Pos{Line: s.End.Line, Col: s.End.Col},
	}
}

func lastSegment(mod string) string {
	mod = strings.TrimSpace(mod)
	if mod == "" {
		return ""
	}
	parts := strings.Split(mod, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}

/* ---------- DMW0002: module imports itself ---------- */

func ScanSelfImport(f *ast.File, module string) []error {
	module = strings.TrimSpace(module)
	if module == "" || f == nil {
		return nil
	}
	var out []error

	code, title := lookupCodeTitle("module", "import_self", "DMW0002", "module imports itself")

	// import <module>
	for _, im := range f.Imports {
		if strings.TrimSpace(im.Path) == module {
			d := diag.Diagnostic{
				Domain:  "module",
				Key:     "import_self",
				Level:   diag.LevelWarning,
				Code:    code,
				Message: fmt.Sprintf("%s: %q", title, module),
				Span:    spanFromAST(im.Span),
			}
			out = append(out, d)
		}
	}

	// from <module> import ...
	for _, fm := range f.FromImports {
		if strings.TrimSpace(fm.Module) == module {
			d := diag.Diagnostic{
				Domain:  "module",
				Key:     "import_self",
				Level:   diag.LevelWarning,
				Code:    code,
				Message: fmt.Sprintf("%s (from-import): %q", title, module),
				Span:    spanFromAST(fm.Span),
			}
			out = append(out, d)
		}
	}

	return out
}

/* ---------- DMW0003: import alias conflict ---------- */

type aliasSrc struct {
	kind string // "import" or "from-import"
	desc string // human-readable source, e.g. "import foo.bar as io"
	span diag.Span
}

func ScanImportAliasConflicts(f *ast.File) []error {
	if f == nil {
		return nil
	}
	// Collect all locally introduced names from imports (defaulted or explicit alias).
	byAlias := map[string][]aliasSrc{}

	// import decls
	for _, im := range f.Imports {
		local := strings.TrimSpace(im.As)
		if local == "" {
			local = lastSegment(im.Path)
		}
		if local == "" {
			continue
		}
		src := aliasSrc{
			kind: "import",
			desc: func() string {
				if strings.TrimSpace(im.As) != "" {
					return fmt.Sprintf("import %s as %s", im.Path, im.As)
				}
				return fmt.Sprintf("import %s", im.Path)
			}(),
			span: spanFromAST(im.Span),
		}
		byAlias[local] = append(byAlias[local], src)
	}

	// from-import decls
	for _, fm := range f.FromImports {
		base := strings.TrimSpace(fm.Module)
		for _, it := range fm.Items {
			local := strings.TrimSpace(it.As)
			if local == "" {
				local = strings.TrimSpace(it.Name)
			}
			if local == "" {
				continue
			}
			src := aliasSrc{
				kind: "from-import",
				desc: func() string {
					if strings.TrimSpace(it.As) != "" {
						return fmt.Sprintf("from %s import %s as %s", base, it.Name, it.As)
					}
					return fmt.Sprintf("from %s import %s", base, it.Name)
				}(),
				span: spanFromAST(it.Span),
			}
			byAlias[local] = append(byAlias[local], src)
		}
	}

	// Emit warnings for any alias that appears more than once.
	if len(byAlias) == 0 {
		return nil
	}
	code, title := lookupCodeTitle("module", "import_alias_conflict", "DMW0003", "import alias conflict")

	var out []error
	for alias, list := range byAlias {
		if len(list) < 2 {
			continue
		}
		// Attach span of the first occurrence; list the rest in notes.
		d := diag.Diagnostic{
			Domain:  "module",
			Key:     "import_alias_conflict",
			Level:   diag.LevelWarning,
			Code:    code,
			Message: fmt.Sprintf("%s: %q", title, alias),
			Span:    list[0].span,
		}
		d.Notes = append(d.Notes, "conflicting sources:")
		for _, s := range list {
			d.Notes = append(d.Notes, "  - "+s.desc)
		}
		out = append(out, d)
	}

	return out
}
