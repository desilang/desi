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
	// HasLocalModule checks if a module exists in local/project loaders (not stdlib).
	HasLocalModule(dotted string) bool
}

/***************
 * MultiLoader *
 ***************/

// multiLoader tries a list of loaders in order until one succeeds.
type multiLoader struct {
	inners      []Loader // file system loaders for local/project modules
	embedStdlib Loader   // embedded stdlib loader (nil if none)
}

func NewFSLoader(root string) *FSLoader {
	return &FSLoader{Root: root}
}

// NewFSLoaderMulti accepts roots in order. Use this for CLI.
// The last roots should be stdlib paths.
func NewFSLoaderMulti(roots []string) Loader {
	return NewFSLoaderMultiWithStdlib(roots, nil)
}

// NewFSLoaderMultiWithStdlib creates a loader with embedded stdlib.
func NewFSLoaderMultiWithStdlib(roots []string, embedStdlib Loader) *multiLoader {
	ml := &multiLoader{embedStdlib: embedStdlib}
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
	// Try local/project loaders first
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
	// Try embedded stdlib last
	if m.embedStdlib != nil {
		mod, diags, err := m.embedStdlib.Load(dotted)
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

// LoadStdlib loads from embedded stdlib only (for std.* imports).
func (m *multiLoader) LoadStdlib(dotted string) (*ast.Module, []diag.Diagnostic, error) {
	if m.embedStdlib == nil {
		// No stdlib configured, fall back to regular load
		return m.Load(dotted)
	}
	return m.embedStdlib.Load(dotted)
}

// HasStdlibModule checks if a module exists in embedded stdlib (for shadow errors).
func (m *multiLoader) HasStdlibModule(dotted string) bool {
	if m.embedStdlib == nil {
		return false
	}
	mod, _, err := m.embedStdlib.Load(dotted)
	return err == nil && mod != nil
}

// HasLocalModule checks if a module exists in local/project loaders (not stdlib).
// Used to ensure shadow warning only triggers when BOTH local and stdlib have the module.
func (m *multiLoader) HasLocalModule(dotted string) bool {
	for _, l := range m.inners {
		mod, _, err := l.Load(dotted)
		if err == nil && mod != nil {
			return true
		}
	}
	return false
}
