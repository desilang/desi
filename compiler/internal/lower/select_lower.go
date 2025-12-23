package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
)

// lowerSelectStmt lowers a select statement.
// Uses a "matched" flag approach: once any case succeeds, skip remaining cases.
func (ls *lowerState) lowerSelectStmt(s *ast.SelectStmt) {
	if len(s.Cases) == 0 {
		// No cases, just run default if present
		for _, stmt := range s.Default {
			ls.lowerStmt(stmt)
		}
		return
	}

	oldCur := ls.b.Block()

	// Create a "matched" flag: starts as false (0)
	matchedFlag := ls.b.FreshTemp("select_matched")
	ls.b.Emit(&hir.Alloca{Type: "i1", Dst: matchedFlag})
	ls.b.Emit(&hir.Store{Dst: matchedFlag, Val: hir.ConstInt{Text: "0", Type: "i1"}})

	for i, sc := range s.Cases {
		// Check if already matched
		alreadyMatched := ls.b.FreshTemp(fmt.Sprintf("already_matched_%d", i))
		ls.b.Emit(&hir.Load{Type: "i1", Src: matchedFlag, Dst: alreadyMatched})

		// Lower the operation only if not already matched
		// Create if block for "not matched" case
		checkBlk := ls.b.NewBlock(fmt.Sprintf("select_check_%d", i))
		ls.b.SetBlock(checkBlk)

		// Lower the operation (try_recv returns ptr)
		opVal := ls.lowerExpr(sc.Op)

		// Check if operation succeeded: convert ptr to i64 and compare != 0
		ptrAsInt := ls.b.FreshTemp(fmt.Sprintf("select_ptr_%d", i))
		ls.b.Emit(&hir.Cast{Src: opVal, Dst: ptrAsInt, Type: "i64"})

		isNotNull := ls.b.FreshTemp(fmt.Sprintf("select_ok_%d", i))
		ls.b.Emit(&hir.BinaryOp{
			Op:   "!=",
			LHS:  ptrAsInt,
			RHS:  hir.ConstInt{Text: "0", Type: "i64"},
			Dst:  isNotNull,
			Type: "i1",
		})

		// If operation succeeded, run body and set matched=true
		bodyBlk := ls.b.NewBlock(fmt.Sprintf("select_body_%d", i))
		ls.b.SetBlock(bodyBlk)
		for _, stmt := range sc.Body {
			ls.lowerStmt(stmt)
		}
		// Set matched flag
		ls.b.Emit(&hir.Store{Dst: matchedFlag, Val: hir.ConstInt{Text: "1", Type: "i1"}})

		ls.b.SetBlock(checkBlk)
		ls.b.Emit(&hir.If{Cond: isNotNull, Then: bodyBlk, Else: nil})

		ls.b.SetBlock(oldCur)

		// Invert alreadyMatched for the outer check
		notMatched := ls.b.FreshTemp(fmt.Sprintf("not_matched_%d", i))
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  alreadyMatched,
			RHS:  hir.ConstInt{Text: "0", Type: "i1"},
			Dst:  notMatched,
			Type: "i1",
		})

		// Only check this case if not matched
		ls.b.Emit(&hir.If{Cond: notMatched, Then: checkBlk, Else: nil})
	}

	// Handle default: run only if no case matched
	if len(s.Default) > 0 {
		finalMatched := ls.b.FreshTemp("final_matched")
		ls.b.Emit(&hir.Load{Type: "i1", Src: matchedFlag, Dst: finalMatched})

		notFinalMatched := ls.b.FreshTemp("not_final_matched")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  finalMatched,
			RHS:  hir.ConstInt{Text: "0", Type: "i1"},
			Dst:  notFinalMatched,
			Type: "i1",
		})

		defaultBlk := ls.b.NewBlock("select_default")
		ls.b.SetBlock(defaultBlk)
		for _, stmt := range s.Default {
			ls.lowerStmt(stmt)
		}
		ls.b.SetBlock(oldCur)

		ls.b.Emit(&hir.If{Cond: notFinalMatched, Then: defaultBlk, Else: nil})
	}
}
