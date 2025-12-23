package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// checkSelectStmt type checks a select statement.
// Validates that case operations are channel recv/send calls.
func (c *checker) checkSelectStmt(s *ast.SelectStmt) {
	for _, sc := range s.Cases {
		// Type check the operation (should be a channel method call)
		opType := c.typ(sc.Op)

		// If there's a binding, introduce it in the case body scope
		if sc.Binding != nil {
			// Save current scope before pushing
			savedScope := c.scope
			// The binding gets the result type of the operation
			c.scope = NewScope(c.scope)
			c.scope.Define(&Symbol{
				Name: sc.Binding.Name,
				Type: opType, // Result of recv operation
			})

			// Type check the case body
			for _, stmt := range sc.Body {
				c.checkStmt(stmt)
			}

			// Restore scope
			c.scope = savedScope
		} else {
			// No binding, just type check body with current scope
			for _, stmt := range sc.Body {
				c.checkStmt(stmt)
			}
		}
	}

	// Type check default body
	for _, stmt := range s.Default {
		c.checkStmt(stmt)
	}
}
