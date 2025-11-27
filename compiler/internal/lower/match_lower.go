package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerMatchExpr(m *ast.MatchExpr) hir.Value {
	// For MVP: match with 2 arms (one variant check + wildcard) only
	// This is a minimal implementation to get basic functionality working

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

	// For MVP: simple 2-arm match (pattern + wildcard)
	// Generate single If statement
	if len(m.Arms) >= 1 {
		arm := m.Arms[0]

		// Check if first arm is a pattern or wildcard
		isWildcard := false
		if id, ok := arm.Pattern.(*ast.Ident); ok && id.Name == "_" {
			isWildcard = true
		}

		if !isWildcard {
			if call, ok := arm.Pattern.(*ast.CallExpr); ok {
				// Enum variant match
				targetTag := -1
				if enumType != nil {
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

				if targetTag != -1 {
					// Compare tag
					cmp := ls.b.FreshTemp("cmp")
					ls.b.Emit(&hir.BinaryOp{
						Op:   "==",
						LHS:  tagVal,
						RHS:  hir.ConstInt{Text: fmt.Sprintf("%d", targetTag)},
						Dst:  cmp,
						Type: "i1",
					})

					// Create then block
					thenBlock := ls.b.NewBlock("match.then")
					savedBlock := ls.b.Block()
					ls.b.SetBlock(thenBlock)
					res := ls.lowerExpr(arm.Result)
					if llvmResType != "void" {
						ls.b.Emit(&hir.Store{Dst: resPtr, Val: res})
					}

					// Create else block (wildcard/default)
					elseBlock := ls.b.NewBlock("match.else")
					ls.b.SetBlock(elseBlock)
					if len(m.Arms) > 1 {
						elseRes := ls.lowerExpr(m.Arms[1].Result)
						if llvmResType != "void" {
							ls.b.Emit(&hir.Store{Dst: resPtr, Val: elseRes})
						}
					}

					// Restore and emit If
					ls.b.SetBlock(savedBlock)
					ls.b.Emit(&hir.If{
						Cond: cmp,
						Then: thenBlock,
						Else: elseBlock,
					})
				}
			}
		} else {
			// Just wildcard - no condition needed
			res := ls.lowerExpr(arm.Result)
			if llvmResType != "void" {
				ls.b.Emit(&hir.Store{Dst: resPtr, Val: res})
			}
		}
	}

	return loadResult(ls, llvmResType, resPtr)
}

func loadResult(ls *lowerState, llvmResType string, resPtr hir.Temp) hir.Value {
	if llvmResType != "void" {
		val := ls.b.FreshTemp("match_val")
		ls.b.Emit(&hir.Load{Type: llvmResType, Src: resPtr, Dst: val})
		return val
	}
	return nil
}
