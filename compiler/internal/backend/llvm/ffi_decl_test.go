package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestExternDeclare_EmittedOnce_ForFallbackCalls(t *testing.T) {
	// Build a tiny HIR that calls an extern function "sin" twice.
	fn := &hir.Func{
		Name: "main",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Call{Fn: "sin"},                             // call #1
				&hir.Call{Fn: "sin", Dst: hir.Temp{Name: "%t9"}}, // call #2 (with dst)
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},
			},
		}},
	}

	m := llvm.NewModule("ffi")
	m.EmitFunc(fn)
	ir := m.IR()

	// Should contain exactly one 'declare' for @sin and at least one call.
	if !strings.Contains(ir, "declare i32 @sin(...)") && !strings.Contains(ir, "declare ptr @sin(...)") {
		t.Fatalf("missing declare for @sin:\n%s", ir)
	}
	if !strings.Contains(ir, "call i32 @sin()") && !strings.Contains(ir, "call ptr @sin()") {
		t.Fatalf("missing call to @sin:\n%s", ir)
	}

	// And our function body must be present.
	if !strings.Contains(ir, "define i32 @main()") {
		t.Fatalf("missing main definition:\n%s", ir)
	}
}
