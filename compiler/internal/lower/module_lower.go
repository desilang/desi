package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
)

// LowerModuleFromSource lowers all top-level function declarations in 'mod'.
// - Synchronous def: 1 HIR func with the same name
// - Async def:       2 HIR funcs: wrapper "<name>" and poll "<name>$poll"
//
// 'src' is used for literal materialization (strings).
// NOTE: For M9B, we also skip functions decorated with @extern(...), even if a body is present.
func LowerModuleFromSource(mod *ast.Module, info *check.Info, src []byte) *hir.Module {
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
			w, p := LowerAsyncFunc(fd, src, info)
			if w == nil || p == nil {
				// Barrier or other early-abort: skip this function, continue module.
				continue
			}
			out.Funcs = append(out.Funcs, w, p)
			continue
		}
		out.Funcs = append(out.Funcs, LowerBlockFromSource(fd.Name.Name, fd.Body, info, src))
	}

	// Lower explicit ImplDecl methods
	for _, d := range mod.Decls {
		if impl, ok := d.(*ast.ImplDecl); ok {
			typeName := impl.ForType.Name
			for _, m := range impl.Methods {
				// Mangle: Type_Method
				name := fmt.Sprintf("%s_%s", typeName, m.Name.Name)
				out.Funcs = append(out.Funcs, LowerBlockFromSource(name, m.Body, info, src))
			}
		}
	}

	// Lower synthesized default Display methods
	if info != nil {
		for _, d := range mod.Decls {
			var typeName string
			if s, ok := d.(*ast.StructDecl); ok {
				typeName = s.Name.Name
			} else if c, ok := d.(*ast.ClassDecl); ok {
				typeName = c.Name.Name
			}

			if typeName != "" {
				if impls, ok := info.Impls[typeName]; ok {
					if methods, ok := impls["Display"]; ok {
						for _, m := range methods {
							// If body is nil, it's the synthesized default
							if m.Name.Name == "to_str" && m.Body == nil {
								name := fmt.Sprintf("%s_to_str", typeName)
								out.Funcs = append(out.Funcs, LowerDefaultToStr(name, typeName))
							}
						}
					}
				}
			}
		}
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
