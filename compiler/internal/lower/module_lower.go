package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
)

// LowerModuleFromSource lowers all top-level function declarations in 'mod'.
// - Synchronous def: 1 HIR func with the same name
// - Async def:       2 HIR funcs: wrapper "<name>" and poll "<name>$poll"
//
// 'src' is used for literal materialization (strings).
// NOTE: For M9B, we also skip functions decorated with @extern(...), even if a body is present.
func LowerModuleFromSource(mod *ast.Module, src []byte) *hir.Module {
	// Async lambdas desugar into hidden __lam$N funcs before normal lowering.
	DesugarAsyncLambdas(mod)

	out := &hir.Module{Name: mod.File}

	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Skip extern-decorated declarations (prototypes) OR body-less declarations.
		if isExternDecorated(fd) || fd.Body == nil {
			continue
		}
		lowerFuncDecl(out, fd)
	}
	return out
}

func isExternDecorated(fd *ast.FuncDecl) bool {
	for _, dec := range fd.Decorators {
		if dec.Name.Name == "extern" {
			return true
		}
	}
	return false
}
