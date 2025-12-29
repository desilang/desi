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

// StdlibLoader extends Loader with stdlib-only loading for std.* imports.
type StdlibLoader interface {
	Loader
	// LoadStdlib loads from stdlib only (for std.X imports).
	LoadStdlib(dotted string) (*ast.Module, []diag.Diagnostic, error)
	// HasStdlibModule checks if a module exists in stdlib (for shadow warnings).
	HasStdlibModule(dotted string) bool
}

/***************
 * MultiLoader *
 ***************/

// multiLoader tries a list of filesystem loaders in order until one succeeds.
type multiLoader struct {
	inners    []*FSLoader
	stdlibIdx int // index where stdlib roots start (-1 if none)
}

func NewFSLoader(root string) *FSLoader {
	return &FSLoader{Root: root}
}

// NewFSLoaderMulti accepts roots in order. Use this for CLI.
// The last roots should be stdlib paths.
func NewFSLoaderMulti(roots []string) Loader {
	return NewFSLoaderMultiWithStdlib(roots, -1)
}

// NewFSLoaderMultiWithStdlib creates a loader with explicit stdlib index.
// stdlibIdx is the index in roots where stdlib begins (-1 for no stdlib).
func NewFSLoaderMultiWithStdlib(roots []string, stdlibIdx int) *multiLoader {
	ml := &multiLoader{stdlibIdx: stdlibIdx}
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

// LoadStdlib loads from stdlib roots only (for std.* imports).
func (m *multiLoader) LoadStdlib(dotted string) (*ast.Module, []diag.Diagnostic, error) {
	if m.stdlibIdx < 0 || m.stdlibIdx >= len(m.inners) {
		// No stdlib configured, fall back to regular load
		return m.Load(dotted)
	}
	var allDiags []diag.Diagnostic
	var lastErr error
	for i := m.stdlibIdx; i < len(m.inners); i++ {
		mod, diags, err := m.inners[i].Load(dotted)
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
	return nil, allDiags, lastErr
}

// HasStdlibModule checks if a module exists in stdlib (for shadow warnings).
func (m *multiLoader) HasStdlibModule(dotted string) bool {
	if m.stdlibIdx < 0 || m.stdlibIdx >= len(m.inners) {
		return false
	}
	for i := m.stdlibIdx; i < len(m.inners); i++ {
		mod, _, err := m.inners[i].Load(dotted)
		if err == nil && mod != nil {
			return true
		}
	}
	return false
}
