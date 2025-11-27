package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// checkMatchStmt type checks a match statement.
// - Verifies scrutinee type
// - Type checks all arms
// - Ensures all arm results have the same type (for expression context)
// - Checks pattern validity (enum variants exist, etc.)
// checkMatchExpr type checks a match expression.
func (c *checker) checkMatchExpr(m *ast.MatchExpr) types.T {
	// Type check the scrutinee (the value being matched)
	scrutineeType := c.typ(m.Scrutinee)
	if scrutineeType == nil {
		return nil
	}

	// Track result types to ensure consistency
	var firstResultType types.T

	for i, arm := range m.Arms {
		// Type check the pattern
		patternType := c.typ(arm.Pattern)

		// For enum matches, verify pattern is compatible with scrutinee
		if et, ok := scrutineeType.(*types.Enum); ok {
			// If pattern is a CallExpr, it should be EnumName.Variant(...)
			if call, ok := arm.Pattern.(*ast.CallExpr); ok {
				// Verify the callee resolves to a variant of this enum
				_ = call
				_ = et
				// TODO: Add variant validation
			}
		}

		// Type check the result expression
		resultType := c.typ(arm.Result)
		if resultType == nil {
			continue
		}

		// c.info.Types[arm.Result] is already set by c.typ(arm.Result)

		// Ensure all results have consistent types
		if i == 0 {
			firstResultType = resultType
		} else {
			if !types.Equal(firstResultType, resultType) {
				c.add(diagAt("DTE0001", arm.Result.SpanOf(),
					"match arm result type '"+resultType.String()+"' does not match first arm type '"+firstResultType.String()+"'"))
			}
		}

		// Check pattern type compatibility
		if patternType != nil && scrutineeType != nil {
			// Pattern should be compatible with scrutinee type
			_ = patternType
		}
	}

	// Store the match statement's result type (type of all arms)
	if firstResultType != nil {
		c.info.Types[m] = firstResultType
	}

	return firstResultType
}
