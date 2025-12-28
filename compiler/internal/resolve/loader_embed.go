package resolve

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

// EmbedFSLoader maps dotted module names -> files within an embedded FS.
// Package rules follow the same conventions as FSLoader.
type EmbedFSLoader struct {
	FS fs.FS
}

// NewEmbedFSLoader creates a loader from an embedded filesystem.
func NewEmbedFSLoader(fsys fs.FS) *EmbedFSLoader {
	return &EmbedFSLoader{FS: fsys}
}

// Load implements Loader for embedded FS.
func (l *EmbedFSLoader) Load(dotted string) (*ast.Module, []diag.Diagnostic, error) {
	if l == nil || l.FS == nil {
		return nil, nil, fmt.Errorf("embed loader has nil FS")
	}

	parts := strings.Split(dotted, ".")

	// Build path for intermediate packages
	cur := ""
	for i := 0; i < len(parts)-1; i++ {
		if cur == "" {
			cur = parts[i]
		} else {
			cur = path.Join(cur, parts[i])
		}
		modPath := path.Join(cur, "__mod.desi")
		if _, err := fs.Stat(l.FS, modPath); err != nil {
			return nil, nil, fmt.Errorf("package not found: %s (missing __mod.desi)", strings.Join(parts[:i+1], "."))
		}
	}

	// Leaf resolution: prefer subpackage, else leaf file
	leaf := parts[len(parts)-1]
	var leafPath string
	if cur == "" {
		leafPath = leaf
	} else {
		leafPath = path.Join(cur, leaf)
	}

	leafPkg := path.Join(leafPath, "__mod.desi")
	leafFile := leafPath + ".desi"

	var filePath string
	if _, err := fs.Stat(l.FS, leafPkg); err == nil {
		filePath = leafPkg
	} else if _, err := fs.Stat(l.FS, leafFile); err == nil {
		filePath = leafFile
	} else {
		return nil, nil, fmt.Errorf("module not found: %s", dotted)
	}

	src, err := fs.ReadFile(l.FS, filePath)
	if err != nil {
		return nil, nil, err
	}

	mod, pdiags := parse.ParseFile(filePath, src)
	return mod, pdiags, nil
}
