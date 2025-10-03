package check

import "github.com/desilang/desi/compiler/internal/ast"

// isConstExpr: Phase B minimal rule — only simple literals count as compile-time constants.
func isConstExpr(e ast.Expr) bool {
	switch e.(type) {
	case *ast.IntLit, *ast.StrLit, *ast.BoolLit:
		return true
	default:
		return false
	}
}

// constKind returns the Kind for a literal expression (Unknown otherwise).
func constKind(e ast.Expr) Kind {
	switch e.(type) {
	case *ast.IntLit:
		return KindInt
	case *ast.StrLit:
		return KindStr
	case *ast.BoolLit:
		return KindBool
	default:
		return KindUnknown
	}
}
