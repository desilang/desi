package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestEmit_Smoke_PrintAndReturn(t *testing.T) {
	fn := &hir.Func{
		Name: "main",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Call{Fn: "print", Args: []hir.Value{hir.ConstStr{Text: "hello world"}}, Dst: hir.Temp{Name: "t1"}},
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},
			},
		}},
	}

	m := llvm.NewModule("test")
	m.EmitFunc(fn)
	ir := m.IR()

	if !strings.Contains(ir, "declare i32 @printf(ptr, ...)") {
		t.Fatalf("missing printf declaration:\n%s", ir)
	}
	if !strings.Contains(ir, "define i32 @main()") || !strings.Contains(ir, "ret i32 0") {
		t.Fatalf("missing function or return:\n%s", ir)
	}
}
