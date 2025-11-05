package lower

import (
	"strconv"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
)

// LowerAsyncFunc lowers an async function declaration into two HIR functions:
//  1. The user-visible wrapper "f" that creates a future, seeds a tiny "frame"
//     (modeled as named locals like "frame.fut", "frame.state"), registers the poller,
//     and returns the future handle.
//  2. A poll function "f$poll(frame*) -> i1" that advances a simple state machine.
//
// M8F generalization: support multiple await suspension points.
// We don't yet model real "save/restore locals"; instead we build a correct multi-state
// skeleton: for each await we set the next state and return pending; the final state
// completes the future. This keeps Tier-0 IR runnable and exercises the shape.
//
// Notes:
// - Borrow barrier across `await` is enforced by the checker (M6).
// - Frame is a lightweight symbol space via "frame.*" names; no struct layout yet.
func LowerAsyncFunc(fd *ast.FuncDecl, _ []byte, _ *check.Info) (wrapper *hir.Func, poll *hir.Func) {
	name := fd.Name.Name

	// Count awaits in the function body (recursively through exprs/statements).
	awCnt := 0
	if fd.Body != nil {
		for _, s := range fd.Body.Stmts {
			awCnt += countAwaitInStmt(s)
		}
	}

	// --- Wrapper: f(args) -> future[T] ---
	wb := hir.NewFunc(name)

	// %fut = future.new
	fut := wb.FreshTemp("t")
	wb.Emit(&hir.FutureNew{Dst: fut})

	// frame.fut = %fut
	wb.Emit(&hir.Let{Name: "frame.fut"})
	wb.Emit(&hir.Assign{LHS: "frame.fut", RHS: fut})

	// frame.state = 0
	wb.Emit(&hir.Let{Name: "frame.state"})
	wb.Emit(&hir.Assign{LHS: "frame.state", RHS: hir.ConstInt{Text: "0"}})

	// call __future_register_poll(%fut, &f$poll, &frame)
	wb.Emit(&hir.Call{
		Fn:   "__future_register_poll",
		Args: []hir.Value{fut, hir.Var{Name: "&" + name + "$poll"}, hir.Var{Name: "&frame"}},
	})

	// ret %fut
	wb.Emit(&hir.Ret{Val: fut})

	// --- Poll: f$poll(frame*) -> i1 ---
	pb := hir.NewFunc(name + "$poll")

	// Create state blocks: state0..state{awCnt}, where the last is completion.
	states := make([]*hir.Block, awCnt+1)
	for i := 0; i <= awCnt; i++ {
		states[i] = hir.NewBlock("state" + strconv.Itoa(i))
	}
	pb.Func().Blocks = append(pb.Func().Blocks, states...)

	// Also create comparison chain blocks for entry (cmp1..cmp{awCnt-1}), to nest the dispatch.
	cmps := make([]*hir.Block, 0, awCnt)
	for i := 1; i < awCnt; i++ {
		cmps = append(cmps, hir.NewBlock("cmp"+strconv.Itoa(i)))
	}
	pb.Func().Blocks = append(pb.Func().Blocks, cmps...)

	// entry: nested comparisons: if state==0 -> state0 else cmp1; cmp1: if state==1 -> state1 else cmp2; ... else state{awCnt}
	// Start emitting in the builder's current block ("entry").
	for i := 0; i < awCnt; i++ {
		// %ti = __eq_i32(frame.state, i)
		ti := pb.FreshTemp("t")
		pb.Emit(&hir.Call{
			Dst:  ti,
			Fn:   "__eq_i32",
			Args: []hir.Value{hir.Var{Name: "frame.state"}, hir.ConstInt{Text: strconv.Itoa(i)}},
		})
		// if %ti then state{i} else (next cmp block or final state)
		var elseBlk *hir.Block
		if i+1 < awCnt {
			elseBlk = cmps[i] // cmp1 for i=0, cmp2 for i=1, ...
		} else {
			elseBlk = states[awCnt]
		}
		pb.Emit(&hir.If{Cond: ti, Then: states[i], Else: elseBlk})

		// Move insertion point into next cmp block (if any) to emit the next comparison.
		if i+1 < awCnt {
			pb.SetBlock(cmps[i])
		}
	}

	// --- Fill each state block ---
	// For i in [0..awCnt-1]: set next state, ret false (Pending)
	for i := 0; i < awCnt; i++ {
		pb.SetBlock(states[i])
		pb.Emit(&hir.Assign{LHS: "frame.state", RHS: hir.ConstInt{Text: strconv.Itoa(i + 1)}})
		pb.Emit(&hir.Ret{Val: hir.ConstBool{Value: false}})
	}

	// Final state: complete and return true (Done)
	pb.SetBlock(states[awCnt])
	pb.Emit(&hir.FutureComplete{
		Fut: hir.Var{Name: "frame.fut"},
		Val: hir.ConstInt{Text: "0"}, // Tier-0: constant payload for tests
	})
	pb.Emit(&hir.Ret{Val: hir.ConstBool{Value: true}})

	return wb.Func(), pb.Func()
}

// ---------- await counting helpers ----------

func countAwaitInStmt(s ast.Stmt) int {
	switch n := s.(type) {
	case *ast.AssignStmt:
		total := 0
		for _, e := range n.RHS {
			total += countAwaitInExpr(e)
		}
		return total

	case *ast.ReturnStmt:
		if n.Value != nil {
			return countAwaitInExpr(n.Value)
		}
		return 0

	case *ast.ExprStmt:
		if n.Expr != nil {
			return countAwaitInExpr(n.Expr)
		}
		return 0

	case *ast.IfStmt:
		total := 0
		// condition
		if n.Cond != nil {
			total += countAwaitInExpr(n.Cond)
		}
		// then
		if n.Then != nil {
			for _, st := range n.Then.Stmts {
				total += countAwaitInStmt(st)
			}
		}
		// elifs
		for _, arm := range n.Elifs {
			if arm.Cond != nil {
				total += countAwaitInExpr(arm.Cond)
			}
			if arm.Body != nil {
				for _, st := range arm.Body.Stmts {
					total += countAwaitInStmt(st)
				}
			}
		}
		// else
		if n.Else != nil {
			for _, st := range n.Else.Stmts {
				total += countAwaitInStmt(st)
			}
		}
		return total

	case *ast.WhileStmt:
		total := 0
		if n.Cond != nil {
			total += countAwaitInExpr(n.Cond)
		}
		if n.Body != nil {
			for _, st := range n.Body.Stmts {
				total += countAwaitInStmt(st)
			}
		}
		return total

	case *ast.ForStmt:
		total := 0
		// Only iterate/Body are relevant for awaits counting.
		if n.Iter != nil {
			total += countAwaitInExpr(n.Iter)
		}
		if n.Body != nil {
			for _, st := range n.Body.Stmts {
				total += countAwaitInStmt(st)
			}
		}
		return total

	default:
		return 0
	}
}

func countAwaitInExpr(e ast.Expr) int {
	switch x := e.(type) {
	case *ast.UnaryExpr:
		// Count 'await' and descend into its operand; for other unary ops just descend.
		if x.Op == "await" {
			return 1 + countAwaitInExpr(x.X)
		}
		return countAwaitInExpr(x.X)

	case *ast.CallExpr:
		total := 0
		for _, a := range x.Args {
			total += countAwaitInExpr(a)
		}
		return total

	case *ast.BinaryExpr:
		return countAwaitInExpr(x.Lhs) + countAwaitInExpr(x.Rhs)

	// No ParenExpr node in this AST; parentheses are not explicitly modeled.

	default:
		return 0
	}
}
