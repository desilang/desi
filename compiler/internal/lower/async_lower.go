package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
)

// LowerAsyncFunc lowers an async function declaration into two HIR functions:
//
//  1. The user-visible wrapper "f" that creates a future, spawns the body
//     function on a background thread, and returns the future handle.
//  2. A body function "f$body" that receives the future + original params
//     and executes the actual function body. The result is returned to the
//     spawn infrastructure which calls __future_complete automatically.
//
// Thread-based model:
//
//	async def add1(x: int) -> int:
//	    return x + 1
//
// Compiles to:
//
//	def add1(x: int) -> Future:      ← wrapper
//	    fut = __future_new()
//	    __future_spawn_1(fut, add1$body, x)
//	    return fut
//
//	def add1$body(fut: ptr, x: int) -> int:   ← body (runs on thread)
//	    return x + 1
func LowerAsyncFunc(fd *ast.FuncDecl, src []byte, info *check.Info, globalNames map[string]bool) (wrapper *hir.Func, body *hir.Func) {
	// Task I: enforce borrow barrier before producing HIR.
	if ds := CheckAwaitBorrowBarrier(fd); len(ds) > 0 {
		return nil, nil
	}

	name := fd.Name.Name

	// --- Wrapper: creates future, spawns body, returns future ---
	wb := hir.NewFunc(name)

	// Copy params from the original function declaration
	for _, p := range fd.Params {
		wb.Func().Params = append(wb.Func().Params, hir.Param{Name: p.Name.Name})
	}

	// Create future
	fut := wb.FreshTemp("fut")
	wb.Emit(&hir.FutureNew{Dst: fut})

	// Build spawn: FutureSpawn encodes the types directly so LLVM emits correct IR
	var spawnArgs []hir.Value
	for _, p := range fd.Params {
		spawnArgs = append(spawnArgs, hir.Var{Name: p.Name.Name})
	}

	wb.Emit(&hir.FutureSpawn{
		Fut:    fut,
		BodyFn: name + "$body",
		Args:   spawnArgs,
	})

	wb.Emit(&hir.Ret{Val: fut})

	// --- Body: the actual function code ---
	// The body function receives (future_ptr, original_params...) and returns
	// the result value. The C runtime's spawn infrastructure calls
	// __future_complete automatically with the return value.
	bodyFunc := LowerFuncFromDeclEx(fd, info, src, globalNames, false)
	if bodyFunc == nil {
		// Fallback: create minimal body
		bb := hir.NewFunc(name + "$body")
		bb.Func().Params = append(bb.Func().Params, hir.Param{Name: "__future__"})
		for _, p := range fd.Params {
			bb.Func().Params = append(bb.Func().Params, hir.Param{Name: p.Name.Name})
		}
		bb.Emit(&hir.Ret{Val: hir.ConstInt{Text: "0"}})
		return wb.Func(), bb.Func()
	}

	// Rename the body function and prepend the future param
	bodyFunc.Name = name + "$body"
	bodyFunc.Params = append([]hir.Param{{Name: "__future__"}}, bodyFunc.Params...)

	return wb.Func(), bodyFunc
}

// itoa converts a small int to string without importing strconv.
func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	return itoa(n/10) + string(digits[n%10])
}
