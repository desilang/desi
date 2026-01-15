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
		} else if fn, ok := scrutineeType.(*types.Func); ok {
			// Unit variant case: Status.Pending has type () -> Status
			// Extract enum from function return type
			if e, ok := fn.Ret.(*types.Enum); ok {
				et = e
			} else if g, ok := fn.Ret.(*types.Generic); ok {
				if e, ok := g.Base.(*types.Enum); ok {
					et = e
					typeArgs = g.Args
				}
			}
		}

		if et != nil {
			if call, ok := arm.Pattern.(*ast.CallExpr); ok {
				// Pattern is EnumName.Variant(args...) OR just Variant(args...)
				// Extract variant name
				var variantName string
				var variant *types.Variant

				if sel, ok := call.Callee.(*ast.FieldExpr); ok {
					// Qualified: Data.Text(s)
					variantName = sel.Name.Name
					// Find variant in enum
					for k := range et.Variants {
						if et.Variants[k].Name == variantName {
							variant = &et.Variants[k]
							break
						}
					}
				} else if id, ok := call.Callee.(*ast.Ident); ok {
					// Unqualified: Text(s) - resolve using scrutinee's enum type
					variantName = id.Name
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
							// Determine type of the field
							fieldType := variant.Fields[j].Type

							// If we have type arguments, substitute them
							if len(subst) > 0 {
								fieldType = substitute(fieldType, subst)
							}

							// Argument can be an identifier (binding) or CallExpr (nested pattern)
							switch argExpr := arg.(type) {
							case *ast.Ident:
								// Simple binding (e.g., v in Some(v))
								// Don't bind wildcards
								if argExpr.Name == "_" {
									continue
								}

								// Create binding
								binding := MatchBinding{
									Name:       argExpr.Name,
									Type:       fieldType,
									FieldIndex: j,
									Node:       argExpr,
								}
								bindings = append(bindings, binding)

								// Add to temporary scope for arm body
								sym := &Symbol{
									Name: argExpr.Name,
									Kind: SymVar,
									Type: fieldType,
									Node: argExpr,
								}
								c.info.Idents[argExpr] = sym

							case *ast.CallExpr:
								// Nested pattern (e.g., Some(v) in Some(Some(v)))
								// Extract variant from nested pattern
								var nestedVariantName string
								if nestedIdent, ok := argExpr.Callee.(*ast.Ident); ok {
									nestedVariantName = nestedIdent.Name
								} else if nestedField, ok := argExpr.Callee.(*ast.FieldExpr); ok {
									nestedVariantName = nestedField.Name.Name
								}

								// Get inner enum type from fieldType
								var innerEt *types.Enum
								var innerTypeArgs []types.T
								if e, ok := fieldType.(*types.Enum); ok {
									innerEt = e
								} else if g, ok := fieldType.(*types.Generic); ok {
									if e, ok := g.Base.(*types.Enum); ok {
										innerEt = e
										innerTypeArgs = g.Args
									}
								}

								if innerEt != nil && nestedVariantName != "" {
									// Find the nested variant
									var nestedVariant *types.Variant
									for k := range innerEt.Variants {
										if innerEt.Variants[k].Name == nestedVariantName {
											nestedVariant = &innerEt.Variants[k]
											break
										}
									}

									if nestedVariant != nil && len(argExpr.Args) > 0 {
										// Recursively process nested pattern args
										innerSubst := make(map[string]types.T)
										if len(innerTypeArgs) > 0 && len(innerEt.TypeParams) > 0 {
											for k, tp := range innerEt.TypeParams {
												if k < len(innerTypeArgs) {
													innerSubst[tp.Name] = innerTypeArgs[k]
												}
											}
										}

										for k, nestedArg := range argExpr.Args {
											if nestedIdent, ok := nestedArg.(*ast.Ident); ok && nestedIdent.Name != "_" {
												// Get the inner field type
												innerFieldType := nestedVariant.Fields[k].Type
												if len(innerSubst) > 0 {
													innerFieldType = substitute(innerFieldType, innerSubst)
												}

												// Create binding with nested pattern info
												binding := MatchBinding{
													Name:          nestedIdent.Name,
													Type:          innerFieldType,
													FieldIndex:    j, // Outer field index
													Node:          nestedIdent,
													NestedPattern: argExpr,
													NestedType:    fieldType,
												}
												bindings = append(bindings, binding)

												// Add to scope
												sym := &Symbol{
													Name: nestedIdent.Name,
													Kind: SymVar,
													Type: innerFieldType,
													Node: nestedIdent,
												}
												c.info.Idents[nestedIdent] = sym
											}
										}
									}
								}
							default:
								c.add(diagAt("DTE0001", arg.SpanOf(),
									"pattern binding must be an identifier or nested pattern"))
							}
						}
					}
				}
			}
		}

		// Handle struct pattern: Point(x, y) when scrutinee is struct type
		if len(bindings) == 0 {
			var st *types.Struct
			if s, ok := scrutineeType.(*types.Struct); ok {
				st = s
			}

			if st != nil {
				if call, ok := arm.Pattern.(*ast.CallExpr); ok {
					// Check if callee matches struct name
					if id, ok := call.Callee.(*ast.Ident); ok && id.Name == st.Name {
						// Struct pattern with field bindings
						// Arguments bind to struct fields in order
						for j, arg := range call.Args {
							if j >= len(st.Fields) {
								continue
							}
							field := st.Fields[j]

							if ident, ok := arg.(*ast.Ident); ok && ident.Name != "_" {
								binding := MatchBinding{
									Name:       ident.Name,
									Type:       field.Type,
									FieldIndex: j,
									Node:       ident,
								}
								bindings = append(bindings, binding)

								sym := &Symbol{
									Name: ident.Name,
									Kind: SymVar,
									Type: field.Type,
									Node: ident,
								}
								c.info.Idents[ident] = sym
							}
						}
					}
				}
			}
		}

	skipBindings:
		// Handle identifier pattern bindings for guards (e.g., n if n > 0)
		// If pattern is a simple identifier (not _ or enum variant), bind it to scrutinee type
		if len(bindings) == 0 && arm.Guard != nil {
			if id, ok := arm.Pattern.(*ast.Ident); ok && id.Name != "_" {
				// Create binding: identifier binds to scrutinee value
				binding := MatchBinding{
					Name:       id.Name,
					Type:       scrutineeType,
					FieldIndex: -1, // Not an enum field
					Node:       id,
				}
				bindings = append(bindings, binding)
			}
		}

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
		// Also skip for any patterns that look like variant patterns (Ident or Ident(...))
		// These are pattern syntax, not expressions, so they shouldn't be type-checked
		skipPatternCheck := len(bindings) > 0
		if !skipPatternCheck {
			// Check if pattern looks like an unqualified variant reference
			if call, ok := arm.Pattern.(*ast.CallExpr); ok {
				if _, ok := call.Callee.(*ast.Ident); ok {
					// Ident(args) pattern - assume it's a variant, don't type-check
					skipPatternCheck = true
				}
			} else if id, ok := arm.Pattern.(*ast.Ident); ok && id.Name != "_" && et != nil {
				// Bare Ident pattern - only skip if it matches a variant
				for _, v := range et.Variants {
					if v.Name == id.Name {
						skipPatternCheck = true
						break
					}
				}
			}
		}
		if !skipPatternCheck {
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

		// Type check guard expression if present (must be bool)
		if arm.Guard != nil {
			guardType := c.typ(arm.Guard)
			if guardType != nil && !types.Equal(guardType, types.Bool) {
				c.add(diagAt("DTE0001", arm.Guard.SpanOf(),
					"guard expression must be bool, got '"+guardType.String()+"'"))
			}
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
			if firstResultType != nil && !types.Equal(firstResultType, resultType) {
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
					// Qualified: Data.Text(s)
					variantName = sel.Name.Name
				} else if id, ok := call.Callee.(*ast.Ident); ok {
					// Unqualified: Text(s)
					variantName = id.Name
				}
			} else if sel, ok := arm.Pattern.(*ast.FieldExpr); ok {
				// Qualified unit: Data.Empty
				variantName = sel.Name.Name
			} else if id, ok := arm.Pattern.(*ast.Ident); ok && id.Name != "_" {
				// Unqualified unit: Empty (check if it's a variant name)
				for _, v := range et.Variants {
					if v.Name == id.Name {
						variantName = id.Name
						break
					}
				}
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
				c.add(diagAt("DTE0052", m.Span, msg))
			}
		}
	}

	return firstResultType
}
