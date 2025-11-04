package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestEmit_AsyncStubs_SyncAwait(t *testing.T) {
	fn := &hir.Func{
		Name: "main",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.FutureNew{Dst: hir.Temp{Name: "%f"}},
				&hir.Await{Fut: hir.Temp{Name: "%f"}, Dst: hir.Temp{Name: "%v"}},
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},
			},
		}},
	}
	m := llvm.NewModule("test")
	m.EmitFunc(fn)
	ir := m.IR()

	// Declarations
	if !strings.Contains(ir, "declare ptr @__future_new()") {
		t.Fatalf("missing future_new declaration:\n%s", ir)
	}
	if !strings.Contains(ir, "declare i32 @__await_blocking(ptr)") {
		t.Fatalf("missing await_blocking declaration:\n%s", ir)
	}

	// Calls
	if !strings.Contains(ir, " = call ptr @__future_new()") {
		t.Fatalf("missing call to __future_new:\n%s", ir)
	}
	if !strings.Contains(ir, " = call i32 @__await_blocking(ptr %f)") {
		t.Fatalf("missing call to __await_blocking(ptr %f):\n%s", ir)
	}
}
