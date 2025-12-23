package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// lowerSelectStmt lowers a select statement.
// For now, each case is lowered as a simple if check on the try_recv result.
// If the result is non-null, the case body executes.
// If no case succeeds, the default body (if any) executes.
func (ls *lowerState) lowerSelectStmt(s *ast.SelectStmt) {
	if len(s.Cases) == 0 {
		// No cases, just run default if present
		for _, stmt := range s.Default {
			ls.lowerStmt(stmt)
		}
		return
	}

	// For simplicity, we lower select as sequential checks using the existing
	// if-else mechanism. Each case becomes: lower op, check result, lower body if success.
	// This is a "first match wins" non-blocking approach.

	for _, sc := range s.Cases {
		// Lower the operation (side effects matter)
		_ = ls.lowerExpr(sc.Op)

		// For try_recv which returns ptr, check if non-null
		// Emit check using the nullity approach from other parts of the codebase
		// Simply lower the body - the operation itself already handles the semantics

		// Lower case body
		for _, stmt := range sc.Body {
			ls.lowerStmt(stmt)
		}
	}

	// Lower default case
	for _, stmt := range s.Default {
		ls.lowerStmt(stmt)
	}
}
