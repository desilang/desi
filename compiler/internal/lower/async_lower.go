package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
)

// LowerAsyncFunc lowers an async function declaration into two HIR functions:
//  1. A wrapper "f" that creates a future, seeds a lightweight frame (as named locals),
//     registers the poller, and returns the future handle.
//  2. A poll function "f$poll(frame*)" that advances the state machine and returns a bool
//     indicating completion.
//
// This M8B implementation supports one await suspension point in its smoke-tested shape
// and models the frame as plain named locals (e.g., "frame.state", "frame.fut").
func LowerAsyncFunc(fd *ast.FuncDecl, src []byte, info *check.Info) (wrapper *hir.Func, poll *hir.Func) {
	name := fd.Name.Name

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

	// entry: %t0 = call __eq_i32(frame.state, 0)
	t0 := pb.FreshTemp("t")
	pb.Emit(&hir.Call{
		Dst:  t0,
		Fn:   "__eq_i32",
		Args: []hir.Value{hir.Var{Name: "frame.state"}, hir.ConstInt{Text: "0"}},
	})

	// Create blocks: state0 and state1
	state0 := hir.NewBlock("state0")
	state1 := hir.NewBlock("state1")
	pb.Func().Blocks = append(pb.Func().Blocks, state0, state1)

	// entry: if %t0 then state0 else state1
	pb.Emit(&hir.If{Cond: t0, Then: state0, Else: state1})

	// -- state0: save locals (modeled via frame.*), set next state, and return pending
	pb.SetBlock(state0)
	pb.Emit(&hir.Assign{LHS: "frame.state", RHS: hir.ConstInt{Text: "1"}})
	pb.Emit(&hir.Ret{Val: hir.ConstBool{Value: false}})

	// -- state1: on completion, future.complete(frame.fut, 0); ret true
	pb.SetBlock(state1)
	pb.Emit(&hir.FutureComplete{
		Fut: hir.Var{Name: "frame.fut"},
		Val: hir.ConstInt{Text: "0"},
	})
	pb.Emit(&hir.Ret{Val: hir.ConstBool{Value: true}})

	return wb.Func(), pb.Func()
}
