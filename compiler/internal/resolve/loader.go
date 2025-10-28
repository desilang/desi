package resolve

import (
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// Loader abstracts a module source.
type Loader interface {
	// Load parses and returns the module for a dotted path (e.g., "math" or "util.math").
	// On parse issues, it returns structured diagnostics in the second result.
	Load(dotted string) (*ast.Module, []diag.Diagnostic, error)
}

/***************
 * MultiLoader *
 ***************/

// multiLoader tries a list of filesystem loaders in order until one succeeds.
type multiLoader struct {
	inners []*FSLoader
}

func NewFSLoader(root string) *FSLoader {
	return &FSLoader{Root: root}
}

// NewFSLoaderMulti accepts colon- or OS-ListPath-like roots. Use this for CLI.
func NewFSLoaderMulti(roots []string) Loader {
	ml := &multiLoader{}
	for _, r := range roots {
		if r == "" {
			continue
		}
		ml.inners = append(ml.inners, NewFSLoader(filepath.Clean(r)))
	}
	return ml
}

func (m *multiLoader) Load(dotted string) (*ast.Module, []diag.Diagnostic, error) {
	var allDiags []diag.Diagnostic
	var lastErr error
	for _, l := range m.inners {
		mod, diags, err := l.Load(dotted)
		if err == nil && mod != nil {
			return mod, diags, nil
		}
		if len(diags) > 0 {
			allDiags = append(allDiags, diags...)
		}
		if err != nil {
			lastErr = err
		}
	}
	// normalize dotted for error context
	_ = strings.ReplaceAll(dotted, "/", ".")
	return nil, allDiags, lastErr
}
