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
	cond := ls.buildMatchCondition(arm.Pattern, tagVal, enumType)

	if cond == nil {
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
				// For MVP: single field payload, so just load directly
				val := ls.b.FreshTemp(binding.Name)
				ls.b.Emit(&hir.Load{
					Type: lowerType(binding.Type),
					Src:  payloadPtr,
					Dst:  val,
				})

				// Store in matchLocals for use in arm body
				ls.matchLocals[binding.Name] = val
			}
		}
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
	elseBlock := ls.b.NewBlock(fmt.Sprintf("match.else%d", start))
	ls.b.SetBlock(elseBlock)

	// Recursively handle remaining arms in else block
	ls.lowerMatchArms(m, arms, start+1, scrutinee, tagVal, enumType, resPtr, llvmResType)

	// Go back to original block and emit If
	ls.b.SetBlock(savedBlock)
	ls.b.Emit(&hir.If{
		Cond: cond,
		Then: thenBlock,
		Else: elseBlock,
	})
}

// buildMatchCondition creates the condition expression for a pattern match
func (ls *lowerState) buildMatchCondition(pattern ast.Expr, tagVal hir.Value, enumType *types.Enum) hir.Value {
	// For now, only support enum variant patterns
	var variantName string

	if call, ok := pattern.(*ast.CallExpr); ok {
		// Enum variant match: Enum.Variant(...)
		if sel, ok := call.Callee.(*ast.FieldExpr); ok {
			variantName = sel.Name.Name
		}
	} else if sel, ok := pattern.(*ast.FieldExpr); ok {
		// Enum variant match: Enum.Variant
		variantName = sel.Name.Name
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
