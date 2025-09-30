package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

func (c *checker) structNameOfExpr(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.StructLit:
		// Direct struct literal: we know its name.
		return v.Name
	case *ast.IdentExpr:
		if vi, ok := c.scope.lookup(v.Name); ok && vi.kind == KindStruct {
			return vi.structName
		}
	}
	return ""
}

/* ---------- misc helpers ---------- */

func isNoneText(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "", "none", "void":
		return true
	default:
		return false
	}
}
