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
	// Task I: enforce barrier before producing HIR.
	if ds := CheckAwaitBorrowBarrier(fd); len(ds) > 0 {
		// Abort lowering for this function; caller should skip adding nils.
		return nil, nil
	}

	name := fd.Name.Name
	awCnt := 0
	if fd.Body != nil {
		for _, s := range fd.Body.Stmts {
			awCnt += countAwaitInStmt(s)
		}
	}

	// --- Wrapper ---
	wb := hir.NewFunc(name)
	fut := wb.FreshTemp("t")
	wb.Emit(&hir.FutureNew{Dst: fut})

	wb.Emit(&hir.Let{Name: "frame.fut"})
	wb.Emit(&hir.Assign{LHS: "frame.fut", RHS: fut})
	wb.Emit(&hir.Let{Name: "frame.state"})
	wb.Emit(&hir.Assign{LHS: "frame.state", RHS: hir.ConstInt{Text: "0"}})

	wb.Emit(&hir.Call{
		Fn:   "__future_register_poll",
		Args: []hir.Value{fut, hir.Var{Name: "&" + name + "$poll"}},
	})
	wb.Emit(&hir.Ret{Val: fut})

	// --- Poll ---
	pb := hir.NewFunc(name + "$poll")
	pb.Func().Params = []hir.Param{{Name: "frame"}}

	states := make([]*hir.Block, awCnt+1)
	for i := 0; i <= awCnt; i++ {
		states[i] = hir.NewBlock("state" + strconv.Itoa(i))
	}
	pb.Func().Blocks = append(pb.Func().Blocks, states...)

	cmps := make([]*hir.Block, 0, awCnt)
	for i := 1; i < awCnt; i++ {
		cmps = append(cmps, hir.NewBlock("cmp"+strconv.Itoa(i)))
	}
	pb.Func().Blocks = append(pb.Func().Blocks, cmps...)

	for i := 0; i < awCnt; i++ {
		ti := pb.FreshTemp("t")
		pb.Emit(&hir.Call{
			Dst:  ti,
			Fn:   "__eq_i32",
			Args: []hir.Value{hir.Var{Name: "frame.state"}, hir.ConstInt{Text: strconv.Itoa(i)}},
		})
		var elseBlk *hir.Block
		if i+1 < awCnt {
			elseBlk = cmps[i]
		} else {
			elseBlk = states[awCnt]
		}
		pb.Emit(&hir.If{Cond: ti, Then: states[i], Else: elseBlk})
		if i+1 < awCnt {
			pb.SetBlock(cmps[i])
		}
	}

	for k := 1; k < len(states); k++ {
		pb.SetBlock(states[k])
		tf := pb.FreshTemp("t")
		pb.Emit(&hir.FrameGet{Slot: "frame.fut", Dst: tf})
		_ = tf
	}

	for i := 0; i < awCnt; i++ {
		pb.SetBlock(states[i])
		pb.Emit(&hir.FrameSet{Slot: "frame.fut", Val: hir.ConstInt{Text: "0"}})
		pb.Emit(&hir.Assign{LHS: "frame.state", RHS: hir.ConstInt{Text: strconv.Itoa(i + 1)}})
		pb.Emit(&hir.Ret{Val: hir.ConstBool{Value: false}})
	}

	pb.SetBlock(states[awCnt])
	pb.Emit(&hir.FutureComplete{
		Fut: hir.Var{Name: "frame.fut"},
		Val: hir.ConstInt{Text: "0"},
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
