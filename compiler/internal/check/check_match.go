package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// checkMatchExpr type checks a match expression.
// - Verifies scrutinee type
// - Type checks all arms
// - Validates pattern variable bindings
// - Ensures all arm results have the same type
func (c *checker) checkMatchExpr(m *ast.MatchExpr) types.T {
	// Type check the scrutinee (the value being matched)
	scrutineeType := c.typ(m.Scrutinee)
	if scrutineeType == nil {
		return nil
	}

	// Track result types to ensure consistency
	var firstResultType types.T

	for i, arm := range m.Arms {
		// Handle pattern binding for enum variants
		var bindings []MatchBinding

		// Unwrap Generic to get Enum and type arguments
		var et *types.Enum
		var typeArgs []types.T

		if e, ok := scrutineeType.(*types.Enum); ok {
			et = e
		} else if g, ok := scrutineeType.(*types.Generic); ok {
			if e, ok := g.Base.(*types.Enum); ok {
				et = e
				typeArgs = g.Args
			}
		}

		if et != nil {
			if call, ok := arm.Pattern.(*ast.CallExpr); ok {
				// Pattern is EnumName.Variant(args...)
				// Extract variant name
				var variantName string
				var variant *types.Variant

				if sel, ok := call.Callee.(*ast.FieldExpr); ok {
					variantName = sel.Name.Name
					// Find variant in enum
					for k := range et.Variants {
						if et.Variants[k].Name == variantName {
							variant = &et.Variants[k]
							break
						}
					}
				}

				if variant != nil {
					// Validate binding arguments
					if len(call.Args) > 0 {
						// Check arity
						if len(call.Args) != len(variant.Fields) {
							c.add(diagAt("DTE0001", arm.Pattern.SpanOf(),
								"pattern arity mismatch: variant has "+string(rune(len(variant.Fields)))+" field(s), got "+string(rune(len(call.Args)))))
							goto skipBindings
						}

						// Prepare substitution map if needed
						subst := make(map[string]types.T)
						if len(typeArgs) > 0 && len(et.TypeParams) > 0 {
							// et.TypeParams is []types.TypeParam (from types.Enum definition)
							// Assuming et.TypeParams matches typeArgs length
							for i, tp := range et.TypeParams {
								if i < len(typeArgs) {
									subst[tp.Name] = typeArgs[i]
								}
							}
						}

						// Validate each binding
						for j, arg := range call.Args {
							// Argument must be an identifier
							ident, ok := arg.(*ast.Ident)
							if !ok {
								c.add(diagAt("DTE0001", arg.SpanOf(),
									"pattern binding must be an identifier"))
								continue
							}

							// Don't bind wildcards
							if ident.Name == "_" {
								continue
							}

							// Determine type of the field
							fieldType := variant.Fields[j].Type

							// If we have type arguments, substitute them
							if len(subst) > 0 {
								fieldType = substitute(fieldType, subst)
							}

							// Create binding
							binding := MatchBinding{
								Name:       ident.Name,
								Type:       fieldType,
								FieldIndex: j,
								Node:       ident,
							}
							bindings = append(bindings, binding)

							// Add to temporary scope for arm body
							// Create a symbol for this binding
							sym := &Symbol{
								Name: ident.Name,
								Kind: SymVar,
								Type: fieldType,
								Node: ident,
							}
							c.info.Idents[ident] = sym
						}
					}
				}
			}
		}

	skipBindings:
		// Store bindings for this arm
		if len(bindings) > 0 {
			if c.info.MatchBindings[m] == nil {
				c.info.MatchBindings[m] = make(map[int][]MatchBinding)
			}
			c.info.MatchBindings[m][i] = bindings
		}

		// Type check the pattern (for validation)
		// NOTE: Skip this for patterns with bindings, since we've already validated them
		// and c.typ would try to evaluate binding arguments as expressions
		if len(bindings) == 0 {
			_ = c.typ(arm.Pattern)
		}

		// Type check the result expression (can now use bindings)
		// Push scope for bindings
		c.scope = NewScope(c.scope)
		for _, b := range bindings {
			sym := &Symbol{
				Name: b.Name,
				Kind: SymVar,
				Type: b.Type,
				Node: b.Node,
			}
			c.scope.Define(sym)
		}

		resultType := c.typ(arm.Result)
		c.scope = c.scope.parent
		if resultType == nil {
			continue
		}

		// Ensure all results have consistent types
		if i == 0 {
			firstResultType = resultType
		} else {
			if firstResultType != nil && resultType != nil && !types.Equal(firstResultType, resultType) {
				c.add(diagAt("DTE0001", arm.Result.SpanOf(),
					"match arm result type '"+resultType.String()+"' does not match first arm type '"+firstResultType.String()+"'"))
			}
		}
	}

	// Store the match expression's result type
	if firstResultType != nil {
		c.info.Types[m] = firstResultType
	}

	// Exhaustiveness check
	if et, ok := scrutineeType.(*types.Enum); ok {
		covered := make(map[string]bool)
		hasWildcard := false

		for _, arm := range m.Arms {
			// Check for wildcard
			if id, ok := arm.Pattern.(*ast.Ident); ok && id.Name == "_" {
				hasWildcard = true
				break
			}

			// Check for variant match
			var variantName string
			if call, ok := arm.Pattern.(*ast.CallExpr); ok {
				if sel, ok := call.Callee.(*ast.FieldExpr); ok {
					variantName = sel.Name.Name
				}
			} else if sel, ok := arm.Pattern.(*ast.FieldExpr); ok {
				variantName = sel.Name.Name
			}

			if variantName != "" {
				covered[variantName] = true
			}
		}

		if !hasWildcard {
			var missing []string
			for _, v := range et.Variants {
				if !covered[v.Name] {
					missing = append(missing, v.Name)
				}
			}

			if len(missing) > 0 {
				msg := "match is not exhaustive. Missing variants: " + strings.Join(missing, ", ")
				c.add(diagAt("DW0007", m.Span, msg))
			}
		}
	}

	return firstResultType
}
