package check

import "github.com/desilang/desi/compiler/internal/ast"

// baseAndPath returns the base storage name for an lvalue expression and a
// conservative path string used for future alias refinement. Examples:
//
//	x            -> ("x", "", true)
//	x.f          -> ("x", ".f", true)
//	x.f.g        -> ("x", ".f.g", true)
//	arr[i]       -> ("arr", "[]", true)
//	obj.list[i]  -> ("obj", ".list[]", true)
//
// Non-lvalues return ("", "", false).
func baseAndPath(e ast.Expr) (base string, path string, ok bool) {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name, "", true
	case *ast.FieldExpr:
		if b, p, ok := baseAndPath(x.X); ok {
			seg := "." + x.Name.Name
			return b, p + seg, true
		}
	case *ast.IndexExpr:
		if b, p, ok := baseAndPath(x.X); ok {
			return b, p + "[]", true
		}
	}
	return "", "", false
}

// isLvalue reports whether e is a syntactic lvalue we recognize.
func isLvalue(e ast.Expr) bool {
	_, _, ok := baseAndPath(e)
	return ok
}
