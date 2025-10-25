package resolve

import (
	"fmt"
	"path"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

// MemLoader is an in-memory loader for tests: map[path]=source.
// Keys are POSIX-style paths relative to a notional root, e.g.
//
//	"foo/__mod.desi", "foo/bar/__mod.desi", "foo/bar.desi"
type MemLoader struct {
	Files map[string]string
}

// NewMemLoader constructs a MemLoader. Nil map is treated as empty.
func NewMemLoader(files map[string]string) *MemLoader {
	if files == nil {
		files = map[string]string{}
	}
	return &MemLoader{Files: files}
}

// Load resolves a dotted path under the __mod.desi rules.
// It verifies each intermediate segment is a package (has __mod.desi).
// The final segment may be either a package (__mod.desi) OR a leaf file "name.desi".
func (m *MemLoader) Load(dotted string) (*ast.Module, []diag.Diagnostic, error) {
	parts := splitDotted(dotted)
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("empty module path")
	}

	// Walk intermediates: foo => foo/__mod.desi, foo.bar => foo/__mod.desi, then foo/bar/__mod.desi (if more parts follow).
	cur := ""
	for i := 0; i < len(parts)-1; i++ {
		cur = joinSeg(cur, parts[i])
		pkgFile := path.Join(cur, "__mod.desi")
		src, ok := m.Files[pkgFile]
		if !ok {
			// Intermediate isn't a package.
			return nil, nil, fmt.Errorf("package not found: %s", dotted)
		}
		// We don't need to parse intermediates for Load of the leaf,
		// but parsing is cheap in tests; leave it to Resolve if/when needed.
		_ = src
	}

	// Final segment: prefer subpackage initializer, else leaf file module.
	cur = joinSeg(cur, parts[len(parts)-1])
	tryPkg := path.Join(cur, "__mod.desi")
	if src, ok := m.Files[tryPkg]; ok {
		mod, diags := parse.ParseFile(tryPkg, []byte(src))
		return mod, diags, nil
	}
	tryLeaf := cur + ".desi"
	if src, ok := m.Files[tryLeaf]; ok {
		mod, diags := parse.ParseFile(tryLeaf, []byte(src))
		return mod, diags, nil
	}

	return nil, nil, fmt.Errorf("module not found: %s", dotted)
}

func splitDotted(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

func joinSeg(prefix, seg string) string {
	if prefix == "" {
		return seg
	}
	return path.Join(prefix, seg)
}
