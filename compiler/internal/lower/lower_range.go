package lower

import (
	"strconv"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// intLiteralValue reads an integer literal, including a negated one, which is
// how a descending step is written: range(10, 0, -1).
func intLiteralValue(e ast.Expr) (int64, bool) {
	neg := false
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == "-" {
		neg = true
		e = u.X
	}
	lit, ok := e.(*ast.IntLit)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(lit.Text, 0, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}

func i64Text(n int64) string { return strconv.FormatInt(n, 10) }

// rangeArgs is a range's three bounds already lowered to i64 values, along with
// the step's compile-time value when it is a literal.
type rangeArgs struct {
	start, stop, step hir.Value
	// constStep is the step when it is an integer literal, and stepIsConst says
	// whether it was. A literal step lets the loop pick its comparison at
	// compile time instead of branching on the sign at runtime.
	constStep   int64
	stepIsConst bool
}

// toI64 lowers e and widens it to i64, which is what the range runtime and the
// index loop both work in. Desi ints are i32.
func (ls *lowerState) toI64(e ast.Expr) hir.Value {
	v := ls.lowerExpr(e)
	w := ls.b.FreshTemp("range_i64")
	ls.b.Emit(&hir.Cast{Dst: w, Src: v, Type: "i64"})
	return w
}

// lowerRangeArgs resolves range(stop), range(start, stop) and
// range(start, stop, step) to their three bounds.
func (ls *lowerState) lowerRangeArgs(args []ast.Expr) rangeArgs {
	var ra rangeArgs

	switch len(args) {
	case 1:
		ra.start = hir.ConstInt{Text: "0", Type: "i64"}
		ra.stop = ls.toI64(args[0])
	case 2:
		ra.start = ls.toI64(args[0])
		ra.stop = ls.toI64(args[1])
	default:
		ra.start = ls.toI64(args[0])
		ra.stop = ls.toI64(args[1])
	}

	if len(args) >= 3 {
		// A literal step is the common case and unlocks the fast loop below.
		if lit, ok := intLiteralValue(args[2]); ok {
			ra.constStep = lit
			ra.stepIsConst = lit != 0 // a zero step would never terminate
			ra.step = hir.ConstInt{Text: i64Text(lit), Type: "i64"}
			if lit == 0 {
				// Match the runtime, which clamps 0 to 1 rather than hanging.
				ra.constStep = 1
				ra.stepIsConst = true
				ra.step = hir.ConstInt{Text: "1", Type: "i64"}
			}
		} else {
			ra.step = ls.toI64(args[2])
		}
	} else {
		ra.constStep = 1
		ra.stepIsConst = true
		ra.step = hir.ConstInt{Text: "1", Type: "i64"}
	}

	return ra
}

// emitRangeNew allocates a DesiRange for the first-class uses of range().
func (ls *lowerState) emitRangeNew(args []ast.Expr) hir.Value {
	ra := ls.lowerRangeArgs(args)
	dst := ls.b.FreshTemp("range_obj")
	ls.b.Emit(&hir.Call{
		Dst:  dst,
		Fn:   "range_new",
		Args: []hir.Value{ra.start, ra.stop, ra.step},
		Type: "ptr",
	})
	return dst
}

// tryLowerForRange lowers `for i in range(...)` and `for i in r` where r is a
// range value, returning true when it emitted the loop.
//
// Two shapes, for one reason: a literal range with a literal step — which is
// nearly all real code — becomes a plain counted loop that allocates nothing.
// Anything else (a bound range, or a step only known at runtime) cannot pick
// its comparison at compile time, so it goes through the runtime helpers,
// which already handle direction and length.
func (ls *lowerState) tryLowerForRange(s *ast.ForStmt) bool {
	if ls.info == nil || len(s.Targets) != 1 {
		return false
	}

	call, isCall := s.Iter.(*ast.CallExpr)
	isRangeCall := false
	if isCall {
		if id, ok := call.Callee.(*ast.Ident); ok && id.Name == "range" {
			isRangeCall = len(call.Args) >= 1 && len(call.Args) <= 3
		}
	}

	_, isRangeVal := ls.typeOf(s.Iter).(*types.Range)
	if !isRangeCall && !isRangeVal {
		return false
	}

	if isRangeCall {
		if ra := ls.lowerRangeArgs(call.Args); ra.stepIsConst {
			ls.emitCountedRangeLoop(s, ra)
			return true
		}
		// Dynamic step: fall through to the runtime path, re-lowering the call
		// as a value. lowerRangeArgs above only emitted casts, which are dead.
	}

	ls.emitRuntimeRangeLoop(s, ls.lowerExpr(s.Iter))
	return true
}

// emitCountedRangeLoop is the allocation-free path: the induction variable is
// the range value itself, stepping by a compile-time constant.
func (ls *lowerState) emitCountedRangeLoop(s *ast.ForStmt, ra rangeArgs) {
	// Induction variable, starting at the low bound.
	idxPtr := ls.b.FreshTemp("range_idx_ptr")
	ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
	ls.b.Emit(&hir.Store{Dst: idxPtr, Val: ra.start})

	condBlk := ls.b.NewBlock("range_cond")
	oldCur := ls.b.Block()

	// The step's sign is known, so the comparison is too: counting up stops at
	// the first value >= stop, counting down at the first <= stop.
	cmp := "<"
	if ra.constStep < 0 {
		cmp = ">"
	}

	ls.b.SetBlock(condBlk)
	idxVal := ls.b.FreshTemp("range_idx")
	ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
	condTemp := ls.b.FreshTemp("range_cond_v")
	ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: cmp, LHS: idxVal, RHS: ra.stop, Type: "i1"})
	ls.b.SetBlock(oldCur)

	bodyBlk := ls.b.NewBlock("range_body")
	latchBlk := ls.b.NewBlock("range_latch")
	exitBlk := ls.b.NewBlock("range_exit")
	ls.pushLoop(latchBlk, exitBlk)
	ls.b.SetBlock(bodyBlk)

	idxBody := ls.b.FreshTemp("range_idx_body")
	ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})
	ls.bindRangeTarget(s, idxBody)

	if s.Body != nil {
		ls.lowerBlock(s.Body)
	}
	ls.closeLoopBody(latchBlk)

	incTemp := ls.b.FreshTemp("range_inc")
	ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: ra.step, Type: "i64"})
	ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

	ls.b.SetBlock(oldCur)
	ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk, Exit: exitBlk, Latch: latchBlk})
	ls.b.SetBlock(exitBlk)
}

// emitRuntimeRangeLoop walks 0..range_len and reads each element with
// range_get, so direction and length stay the runtime's business.
func (ls *lowerState) emitRuntimeRangeLoop(s *ast.ForStmt, rangeVal hir.Value) {
	lenTemp := ls.b.FreshTemp("range_len")
	ls.b.Emit(&hir.Call{Dst: lenTemp, Fn: "range_len", Args: []hir.Value{rangeVal}, Type: "i64"})

	idxPtr := ls.b.FreshTemp("range_i_ptr")
	ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
	ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

	condBlk := ls.b.NewBlock("range_cond")
	oldCur := ls.b.Block()

	ls.b.SetBlock(condBlk)
	idxVal := ls.b.FreshTemp("range_i")
	ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
	condTemp := ls.b.FreshTemp("range_cond_v")
	ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
	ls.b.SetBlock(oldCur)

	bodyBlk := ls.b.NewBlock("range_body")
	latchBlk := ls.b.NewBlock("range_latch")
	exitBlk := ls.b.NewBlock("range_exit")
	ls.pushLoop(latchBlk, exitBlk)
	ls.b.SetBlock(bodyBlk)

	idxBody := ls.b.FreshTemp("range_i_body")
	ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})
	elemVal := ls.b.FreshTemp("range_elem")
	ls.b.Emit(&hir.Call{Dst: elemVal, Fn: "range_get", Args: []hir.Value{rangeVal, idxBody}, Type: "i64"})
	ls.bindRangeTarget(s, elemVal)

	if s.Body != nil {
		ls.lowerBlock(s.Body)
	}
	ls.closeLoopBody(latchBlk)

	incTemp := ls.b.FreshTemp("range_i_inc")
	ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
	ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

	ls.b.SetBlock(oldCur)
	ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk, Exit: exitBlk, Latch: latchBlk})
	ls.b.SetBlock(exitBlk)
}

// bindRangeTarget binds the loop variable, narrowing the i64 counter to the
// i32 a Desi int is.
func (ls *lowerState) bindRangeTarget(s *ast.ForStmt, cur hir.Value) {
	if s.Targets[0].Name == nil {
		return
	}
	name := s.Targets[0].Name.Name
	v := ls.b.FreshTemp(name + "_val")
	ls.b.Emit(&hir.Cast{Dst: v, Src: cur, Type: "i32"})
	ls.b.Emit(&hir.Let{Name: name, Init: v, Type: types.Int})
}
