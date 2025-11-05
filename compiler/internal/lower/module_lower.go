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
	// Rewrite async lambdas into hidden async functions before lowering.
	DesugarAsyncLambdas(mod)

	out := &hir.Module{Name: mod.File}
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Skip prototypes (no body) and any function decorated with @extern(...).
		// This lets extern decls survive parsing/resolution but produce no HIR.
		if fd.Body == nil || isExternDecorated(fd) {
			continue
		}

		if fd.Async {
			w, p := LowerAsyncFunc(fd, src, nil)
			if w == nil || p == nil {
				// Barrier or other early-abort: skip this function, continue module.
				continue
			}
			out.Funcs = append(out.Funcs, w, p)
			continue
		}
		out.Funcs = append(out.Funcs, LowerBlockFromSource(fd.Name.Name, fd.Body, src))
	}
	return out
}

// isExternDecorated reports whether the function has an @extern(...) decorator.
func isExternDecorated(fd *ast.FuncDecl) bool {
	for _, dec := range fd.Decorators {
		if dec.Name.Name == "extern" {
			return true
		}
	}
	return false
}
