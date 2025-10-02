package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

// collectImportedNamesInto inspects f.Imports and f.FromImports and records the
// identifiers that become visible in the file's namespace.
func collectImportedNamesInto(dst map[string]bool, f *ast.File) {
	// Plain imports:
	//   import foo[.bar][.baz] [as alias]
	// Record alias if present; otherwise record the FIRST path segment.
	for _, imp := range f.Imports {
		path := strings.TrimSpace(imp.Path)
		if path == "" {
			continue
		}
		if alias := strings.TrimSpace(imp.As); alias != "" {
			dst[alias] = true
			continue
		}
		first := path
		if dot := strings.IndexByte(path, '.'); dot >= 0 {
			first = path[:dot]
		}
		if first != "" {
			dst[first] = true
		}
	}

	// From-imports:
	//   from mod.path import a [as x], b, c [as y]
	for _, fi := range f.FromImports {
		for _, it := range fi.Items {
			name := strings.TrimSpace(it.Name)
			if name == "" {
				continue
			}
			if alias := strings.TrimSpace(it.As); alias != "" {
				dst[alias] = true
			} else {
				dst[name] = true
			}
		}
	}
}
