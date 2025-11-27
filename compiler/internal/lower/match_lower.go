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
	ls.lowerMatchArms(m.Arms, 0, tagVal, enumType, resPtr, llvmResType)

	return loadResult(ls, llvmResType, resPtr)
}

// lowerMatchArms generates If-Else chain for match arms starting from index 'start'
func (ls *lowerState) lowerMatchArms(arms []ast.MatchArm, start int, tagVal hir.Value, enumType *types.Enum, resPtr hir.Temp, llvmResType string) {
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
		ls.lowerMatchArms(arms, start+1, tagVal, enumType, resPtr, llvmResType)
		return
	}

	// Build then block
	thenBlock := ls.b.NewBlock(fmt.Sprintf("match.then%d", start))
	savedBlock := ls.b.Block()
	ls.b.SetBlock(thenBlock)

	res := ls.lowerExpr(arm.Result)
	if llvmResType != "void" {
		ls.b.Emit(&hir.Store{Dst: resPtr, Val: res})
	}

	// Build else block for remaining arms
	elseBlock := ls.b.NewBlock(fmt.Sprintf("match.else%d", start))
	ls.b.SetBlock(elseBlock)

	// Recursively handle remaining arms in else block
	ls.lowerMatchArms(arms, start+1, tagVal, enumType, resPtr, llvmResType)

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
	call, ok := pattern.(*ast.CallExpr)
	if !ok {
		return nil // Unsupported pattern type
	}

	// Enum variant match: Enum.Variant(...)
	targetTag := -1
	if enumType != nil {
		// Extract variant name from CallExpr
		// CallExpr.Callee should be FieldExpr (Enum.Variant)
		if sel, ok := call.Callee.(*ast.FieldExpr); ok {
			variantName := sel.Name.Name
			for _, v := range enumType.Variants {
				if v.Name == variantName {
					targetTag = v.Tag
					break
				}
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
