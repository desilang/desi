package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerMatchExpr(m *ast.MatchExpr) hir.Value {
	// 1. Evaluate scrutinee
	scrutinee := ls.lowerExpr(m.Scrutinee)

	// Get scrutinee type
	scrType := ls.info.Types[m.Scrutinee]

	// Result type of the match expression
	resType := ls.info.Types[m]
	llvmResType := "void"
	if resType != nil {
		llvmResType = lowerType(resType)
	}

	// Create result variable (alloca) if needed
	var resPtr hir.Temp
	if llvmResType != "void" {
		resPtr = ls.b.FreshTemp("match_res")
		ls.b.Emit(&hir.Alloca{Type: llvmResType, Dst: resPtr})
	}

	// If enum, load tag
	var tagVal hir.Value
	var enumType *types.Enum

	if et, ok := scrType.(*types.Enum); ok {
		enumType = et
	} else if g, ok := scrType.(*types.Generic); ok {
		if et, ok := g.Base.(*types.Enum); ok {
			enumType = et
		}
	} else if fn, ok := scrType.(*types.Func); ok {
		// Unit variant case: Data.Empty has type () -> Data
		// Extract enum from function return type
		if et, ok := fn.Ret.(*types.Enum); ok {
			enumType = et
		} else if g, ok := fn.Ret.(*types.Generic); ok {
			if et, ok := g.Base.(*types.Enum); ok {
				enumType = et
			}
		}
	}

	if enumType != nil {
		// Load tag from offset 0
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    scrutinee,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tagTemp := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tagTemp})
		tagVal = tagTemp
	}

	// Handle empty match
	if len(m.Arms) == 0 {
		return loadResult(ls, llvmResType, resPtr)
	}

	// Generate nested If-Else chain recursively
	// Generate nested If-Else chain recursively
	ls.lowerMatchArms(m, m.Arms, 0, scrutinee, tagVal, enumType, resPtr, llvmResType)

	return loadResult(ls, llvmResType, resPtr)
}

// lowerMatchArms generates If-Else chain for match arms starting from index 'start'
func (ls *lowerState) lowerMatchArms(m *ast.MatchExpr, arms []ast.MatchArm, start int, scrutinee hir.Value, tagVal hir.Value, enumType *types.Enum, resPtr hir.Temp, llvmResType string) {
	if start >= len(arms) {
		// No more arms - this shouldn't happen if match is exhaustive
		return
	}

	arm := arms[start]

	// Check for wildcard pattern
	isWildcard := false
	if id, ok := arm.Pattern.(*ast.Ident); ok && id.Name == "_" {
		isWildcard = true
	}

	if isWildcard {
		// Wildcard: unconditional - just execute result
		res := ls.lowerExpr(arm.Result)
		if llvmResType != "void" {
			ls.b.Emit(&hir.Store{Dst: resPtr, Val: res})
		}
		//  No more arms to check after wildcard
		return
	}

	// Pattern match: build condition
	cond := ls.buildMatchCondition(arm.Pattern, scrutinee, tagVal, enumType)

	// Check for identifier pattern with guard (e.g., n if n > 0)
	// This is a catchall pattern that only matches when guard is true
	isIdentifierWithGuard := false
	if cond == nil && arm.Guard != nil {
		if id, ok := arm.Pattern.(*ast.Ident); ok && id.Name != "_" {
			isIdentifierWithGuard = true
		}
	}

	// Check for struct pattern (struct patterns have no tag to check)
	isStructPattern := false
	if cond == nil && enumType == nil {
		scrType := ls.info.Types[m.Scrutinee]
		if _, ok := scrType.(*types.Struct); ok {
			if call, ok := arm.Pattern.(*ast.CallExpr); ok {
				if _, ok := call.Callee.(*ast.Ident); ok {
					isStructPattern = true
				}
			}
		}
	}

	if cond == nil && !isIdentifierWithGuard && !isStructPattern {
		// Pattern not supported, skip to next arm
		ls.lowerMatchArms(m, arms, start+1, scrutinee, tagVal, enumType, resPtr, llvmResType)
		return
	}

	// Build then block
	thenBlock := ls.b.NewBlock(fmt.Sprintf("match.then%d", start))
	savedBlock := ls.b.Block()
	ls.b.SetBlock(thenBlock)

	// Extract payload and bind pattern variables
	// Extract payload and bind pattern variables
	if ls.info != nil && ls.info.MatchBindings[m] != nil {
		bindings := ls.info.MatchBindings[m][start]
		if len(bindings) > 0 && enumType != nil {
			// Load payload pointer from enum (offset 8, aligned after i32 tag + padding)
			payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
			ls.b.Emit(&hir.GetElementPtr{
				Type:    "i8",
				Base:    scrutinee,
				Indices: []hir.Value{hir.ConstInt{Text: "8"}},
				Dst:     payloadPtrSlot,
			})

			payloadPtr := ls.b.FreshTemp("payload_ptr")
			ls.b.Emit(&hir.Load{
				Type: "ptr",
				Src:  payloadPtrSlot,
				Dst:  payloadPtr,
			})

			// For each binding, load the field value
			for _, binding := range bindings {
				val := ls.b.FreshTemp(binding.Name)

				// Check if this is a nested pattern binding
				if binding.NestedPattern != nil {
					// Nested binding: need to extract inner payload
					// payloadPtr is the box pointer, load inner enum ptr from it
					innerEnumPtr := ls.b.FreshTemp("inner_enum_ptr")
					ls.b.Emit(&hir.Load{
						Type: "ptr",
						Src:  payloadPtr,
						Dst:  innerEnumPtr,
					})

					// Load inner payload box pointer (offset 8)
					innerPayloadSlot := ls.b.FreshTemp("inner_payload_slot")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    innerEnumPtr,
						Indices: []hir.Value{hir.ConstInt{Text: "8"}},
						Dst:     innerPayloadSlot,
					})
					innerPayloadBoxPtr := ls.b.FreshTemp("inner_payload_box")
					ls.b.Emit(&hir.Load{Type: "ptr", Src: innerPayloadSlot, Dst: innerPayloadBoxPtr})

					// Load the inner value from the box
					ls.b.Emit(&hir.Load{
						Type: lowerType(binding.Type),
						Src:  innerPayloadBoxPtr,
						Dst:  val,
					})
				} else {
					// Regular binding: load from outer payload
					// Check if binding type is a pointer type (class, struct, list, dict, set)
					isPtr := false
					switch binding.Type.(type) {
					case *types.Class, *types.Struct, *types.List, *types.Dict, *types.Set:
						isPtr = true
					}

					if isPtr {
						// For pointer types, just load the pointer value
						ls.b.Emit(&hir.Load{
							Type: "ptr",
							Src:  payloadPtr,
							Dst:  val,
						})
					} else {
						// For primitive types, load the value
						ls.b.Emit(&hir.Load{
							Type: lowerType(binding.Type),
							Src:  payloadPtr,
							Dst:  val,
						})
					}
				}

				// Store in matchLocals for use in arm body
				ls.matchLocals[binding.Name] = val
			}
		} else if len(bindings) > 0 {
			// Non-enum bindings: identifier pattern or struct pattern
			for _, binding := range bindings {
				if binding.FieldIndex == -1 {
					// Scrutinee binding: the binding IS the scrutinee value
					ls.matchLocals[binding.Name] = scrutinee
				} else if binding.FieldIndex >= 0 {
					// Struct field binding: load field from struct
					val := ls.b.FreshTemp(binding.Name)
					fieldPtr := ls.b.FreshTemp("struct_field_ptr")

					// Get struct type from scrutinee type
					scrType := ls.info.Types[m.Scrutinee]
					if st, ok := scrType.(*types.Struct); ok {
						// Get field offset
						offset := 0
						for i := 0; i < binding.FieldIndex; i++ {
							offset += getSize(st.Fields[i].Type)
						}

						ls.b.Emit(&hir.GetElementPtr{
							Type:    "i8",
							Base:    scrutinee,
							Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
							Dst:     fieldPtr,
						})
						ls.b.Emit(&hir.Load{
							Type: lowerType(binding.Type),
							Src:  fieldPtr,
							Dst:  val,
						})
						ls.matchLocals[binding.Name] = val
					}
				}
			}
		}
	}

	// Build else block for remaining arms (needed for guard fallthrough too)
	elseBlock := ls.b.NewBlock(fmt.Sprintf("match.else%d", start))

	// If there's a guard clause, check it and potentially fall through to else
	if arm.Guard != nil {
		guardVal := ls.lowerExpr(arm.Guard)
		guardThen := ls.b.NewBlock(fmt.Sprintf("guard.then%d", start))

		// Branch: if guard true -> guardThen, else -> elseBlock
		ls.b.Emit(&hir.If{
			Cond: guardVal,
			Then: guardThen,
			Else: elseBlock,
		})
		ls.b.SetBlock(guardThen)
	}

	res := ls.lowerExpr(arm.Result)
	if llvmResType != "void" {
		ls.b.Emit(&hir.Store{Dst: resPtr, Val: res})
	}

	// Clear matchLocals for next arm
	for binding := range ls.matchLocals {
		delete(ls.matchLocals, binding)
	}

	// Build else block for remaining arms
	ls.b.SetBlock(elseBlock)

	// Recursively handle remaining arms in else block
	ls.lowerMatchArms(m, arms, start+1, scrutinee, tagVal, enumType, resPtr, llvmResType)

	// Go back to original block and emit branch
	ls.b.SetBlock(savedBlock)
	if cond != nil {
		ls.b.Emit(&hir.If{
			Cond: cond,
			Then: thenBlock,
			Else: elseBlock,
		})
	} else {
		// Identifier-with-guard: always enter then block
		// (guard will handle the conditional logic, falling through to else if false)
		ls.b.Emit(&hir.If{
			Cond: hir.ConstBool{Value: true},
			Then: thenBlock,
			Else: elseBlock,
		})
	}
}

// buildMatchCondition creates the condition expression for a pattern match
func (ls *lowerState) buildMatchCondition(pattern ast.Expr, scrutinee hir.Value, tagVal hir.Value, enumType *types.Enum) hir.Value {
	// Handle literal patterns (bool, int, str)
	switch lit := pattern.(type) {
	case *ast.BoolLit:
		// Compare scrutinee == literal (bool is i1)
		cmp := ls.b.FreshTemp("cmp")
		litVal := "0"
		if lit.Value {
			litVal = "1"
		}
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  scrutinee,
			RHS:  hir.ConstInt{Text: litVal},
			Dst:  cmp,
			Type: "i1",
		})
		return cmp

	case *ast.IntLit:
		// Compare scrutinee == literal (int is i64)
		cmp := ls.b.FreshTemp("cmp")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  scrutinee,
			RHS:  hir.ConstInt{Text: lit.Text},
			Dst:  cmp,
			Type: "i1",
		})
		return cmp
	}

	// Handle enum variant patterns
	var variantName string

	if call, ok := pattern.(*ast.CallExpr); ok {
		// Enum variant match with payload
		if sel, ok := call.Callee.(*ast.FieldExpr); ok {
			// Qualified: Data.Text(s)
			variantName = sel.Name.Name
		} else if id, ok := call.Callee.(*ast.Ident); ok {
			// Unqualified: Text(s)
			variantName = id.Name
		}
	} else if sel, ok := pattern.(*ast.FieldExpr); ok {
		// Qualified unit variant: Data.Empty
		variantName = sel.Name.Name
	} else if id, ok := pattern.(*ast.Ident); ok && id.Name != "_" && enumType != nil {
		// Unqualified unit variant: Empty (check if it's a variant name)
		for _, v := range enumType.Variants {
			if v.Name == id.Name {
				variantName = id.Name
				break
			}
		}
	} else {
		return nil // Unsupported pattern type
	}

	// Enum variant match
	targetTag := -1
	if enumType != nil && variantName != "" {
		for _, v := range enumType.Variants {
			if v.Name == variantName {
				targetTag = v.Tag
				break
			}
		}
	}

	if targetTag == -1 {
		return nil // Variant not found
	}

	// Generate comparison: tagVal == targetTag
	cmp := ls.b.FreshTemp("cmp")
	ls.b.Emit(&hir.BinaryOp{
		Op:   "==",
		LHS:  tagVal,
		RHS:  hir.ConstInt{Text: fmt.Sprintf("%d", targetTag)},
		Dst:  cmp,
		Type: "i1",
	})

	// Check for nested patterns in CallExpr args
	if call, ok := pattern.(*ast.CallExpr); ok && enumType != nil {
		for _, arg := range call.Args {
			if nestedCall, ok := arg.(*ast.CallExpr); ok {
				// This is a nested pattern - need to add inner tag check
				// For now, we'll add the nested condition in buildMatchCondition recursively

				// Get the variant to determine inner type
				var innerType types.T
				for _, v := range enumType.Variants {
					if v.Name == variantName && len(v.Fields) > 0 {
						innerType = v.Fields[0].Type
						break
					}
				}

				// Extract inner enum type
				var innerEt *types.Enum
				if innerType != nil {
					if e, ok := innerType.(*types.Enum); ok {
						innerEt = e
					} else if g, ok := innerType.(*types.Generic); ok {
						if e, ok := g.Base.(*types.Enum); ok {
							innerEt = e
						}
					}
				}

				if innerEt != nil {
					// Load outer payload box pointer
					payloadPtrSlot := ls.b.FreshTemp("nested_payload_slot")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    scrutinee,
						Indices: []hir.Value{hir.ConstInt{Text: "8"}},
						Dst:     payloadPtrSlot,
					})
					payloadBoxPtr := ls.b.FreshTemp("nested_payload_box")
					ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadPtrSlot, Dst: payloadBoxPtr})

					// Load inner enum ptr from box
					innerEnumPtr := ls.b.FreshTemp("nested_enum_ptr")
					ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadBoxPtr, Dst: innerEnumPtr})

					// Load inner tag
					innerTagPtr := ls.b.FreshTemp("nested_tag_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    innerEnumPtr,
						Indices: []hir.Value{hir.ConstInt{Text: "0"}},
						Dst:     innerTagPtr,
					})
					innerTag := ls.b.FreshTemp("nested_tag")
					ls.b.Emit(&hir.Load{Type: "i32", Src: innerTagPtr, Dst: innerTag})

					// Get inner variant name and tag
					var innerVariantName string
					if id, ok := nestedCall.Callee.(*ast.Ident); ok {
						innerVariantName = id.Name
					} else if sel, ok := nestedCall.Callee.(*ast.FieldExpr); ok {
						innerVariantName = sel.Name.Name
					}

					innerTargetTag := -1
					for _, v := range innerEt.Variants {
						if v.Name == innerVariantName {
							innerTargetTag = v.Tag
							break
						}
					}

					if innerTargetTag >= 0 {
						// Compare inner tag
						innerCmp := ls.b.FreshTemp("nested_cmp")
						ls.b.Emit(&hir.BinaryOp{
							Op:   "==",
							LHS:  innerTag,
							RHS:  hir.ConstInt{Text: fmt.Sprintf("%d", innerTargetTag)},
							Dst:  innerCmp,
							Type: "i1",
						})

						// AND outer and inner conditions
						combined := ls.b.FreshTemp("combined_cmp")
						ls.b.Emit(&hir.BinaryOp{
							Op:   "and",
							LHS:  cmp,
							RHS:  innerCmp,
							Dst:  combined,
							Type: "i1",
						})
						return combined
					}
				}
			}
		}
	}

	return cmp
}

func loadResult(ls *lowerState, llvmResType string, resPtr hir.Temp) hir.Value {
	if llvmResType != "void" {
		val := ls.b.FreshTemp("match_val")
		ls.b.Emit(&hir.Load{Type: llvmResType, Src: resPtr, Dst: val})
		return val
	}
	return nil
}
