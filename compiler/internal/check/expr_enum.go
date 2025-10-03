package check

import "github.com/desilang/desi/compiler/internal/ast"

// enumNameOfExpr: when an expr denotes a value of a particular enum, return its enum name.
func (c *checker) enumNameOfExpr(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.IdentExpr:
		if vi, ok := c.scope.lookup(v.Name); ok && vi.kind == KindEnum {
			return vi.structName
		}
	case *ast.CallExpr:
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				if _, ok := c.info.Enums[id.Name]; ok {
					return id.Name
				}
			}
		}
	}
	return ""
}
