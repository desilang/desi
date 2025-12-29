package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

// FSLoader maps dotted module names -> files under a single root.
// Package rules (Phase-1):
//   - Each intermediate segment MUST be a package directory containing __mod.desi
//   - The leaf may be either <dir>/<leaf>/__mod.desi (subpackage) OR <dir>/<leaf>.desi (leaf file module)
type FSLoader struct {
	Root string
}

// Load implements Loader for a single root.
func (l *FSLoader) Load(dotted string) (*ast.Module, []diag.Diagnostic, error) {
	if l == nil || l.Root == "" {
		return nil, nil, fmt.Errorf("fs loader has empty root")
	}

	parts := strings.Split(dotted, ".")
	cur := l.Root

	// Walk intermediate packages: a, a/b, ...
	for i := 0; i < len(parts)-1; i++ {
		cur = filepath.Join(cur, parts[i])
		mod := filepath.Join(cur, "__mod.desi")
		st, err := os.Stat(mod)
		if err != nil || st.IsDir() {
			return nil, nil, fmt.Errorf("package not found: %s (missing __mod.desi)", strings.Join(parts[:i+1], "."))
		}
	}

	// Leaf resolution: prefer subpackage leaf, else leaf file module.
	// If BOTH exist, that's an ambiguity error.
	leaf := parts[len(parts)-1]
	leafPkg := filepath.Join(cur, leaf, "__mod.desi")
	leafFile := filepath.Join(cur, leaf+".desi")

	pkgExists := false
	fileExists := false
	if st, err := os.Stat(leafPkg); err == nil && !st.IsDir() {
		pkgExists = true
	}
	if st, err := os.Stat(leafFile); err == nil && !st.IsDir() {
		fileExists = true
	}

	// Check for ambiguity: both bar.desi and bar/__mod.desi exist
	if pkgExists && fileExists {
		return nil, []diag.Diagnostic{{
			CodeID:  "DME0011",
			Domain:  "module",
			Message: "ambiguous module '" + leaf + "' - both " + leaf + ".desi and " + leaf + "/ exist",
		}}, fmt.Errorf("ambiguous module: %s", dotted)
	}

	var path string
	if pkgExists {
		path = leafPkg
	} else if fileExists {
		path = leafFile
	} else {
		return nil, nil, fmt.Errorf("module not found: %s", dotted)
	}

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	// IMPORTANT: your parser returns (mod, []diag.Diagnostic) — not []error.
	mod, pdiags := parse.ParseFile(path, src) // see signature in your tree
	return mod, pdiags, nil
}
