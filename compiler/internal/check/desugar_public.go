package check

import "github.com/desilang/desi/compiler/internal/ast"

// DesugarPrecheck runs syntactic desugars that should happen before typing/lowering.
// Today: map(xs,f) / filter(xs,p)  →  list comprehensions.
func DesugarPrecheck(mod *ast.Module) {
	desugarMapFilter(mod)
}
