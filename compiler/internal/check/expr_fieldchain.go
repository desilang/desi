package check

import "github.com/desilang/desi/compiler/internal/ast"

/* ---------- helpers ---------- */

// decomposeFieldExpr flattens a FieldExpr chain a.b.c -> ("a", ["b","c"], true).
func decomposeFieldExpr(e *ast.FieldExpr) (string, []string, bool) {
	var parts []string
	cur := e
	parts = append(parts, cur.Name)
	for {
		if id, ok := cur.X.(*ast.IdentExpr); ok {
			// reverse parts so they are in left-to-right order
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			return id.Name, parts, true
		}
		if fe, ok := cur.X.(*ast.FieldExpr); ok {
			parts = append(parts, fe.Name)
			cur = fe
			continue
		}
		return "", nil, false
	}
}
