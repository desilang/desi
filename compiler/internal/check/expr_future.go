package check

import "github.com/desilang/desi/compiler/internal/ast"

// try to extract Future[T]'s T from common expression shapes.
func (c *checker) futureElemOfExpr(e ast.Expr) Kind {
	switch t := e.(type) {
	case *ast.CallExpr:
		// Ident call: f(...)
		if id, ok := t.Callee.(*ast.IdentExpr); ok {
			name := id.Name
			if orig, ok := c.aliases[name]; ok {
				name = orig
			}
			if sig, ok := c.info.Funcs[name]; ok && sig.Async {
				return sig.RetElem
			}
			return KindUnknown
		}
		// Module alias call: m.f(...)
		if fe, ok := t.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				if _, isAlias := c.modAliases[id.Name]; isAlias {
					if sig, ok := c.info.Funcs[fe.Name]; ok && sig.Async {
						return sig.RetElem
					}
				}
			}
		}
	}
	return KindUnknown
}
