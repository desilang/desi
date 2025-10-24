package resolve

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// Loader abstracts how modules are located and parsed.
// The input is a dotted module path like "foo.bar.baz".
// Implementations must enforce the __mod.desi package rule and allow a leaf file module (foo/bar/baz.desi) as the final segment.
type Loader interface {
	Load(dotted string) (*ast.Module, []diag.Diagnostic, error)
}
