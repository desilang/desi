package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestEmit_LifetimeIntrinsics_ForLocals(t *testing.T) {
	fn := &hir.Func{
		Name: "f",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Let{Name: "a", Init: hir.ConstInt{Text: "1"}},
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},
			},
		}},
	}
	m := llvm.NewModule("test")
	m.EmitFunc(fn)
	ir := m.IR()
	if !strings.Contains(ir, "llvm.lifetime.start") {
		t.Fatalf("missing lifetime.start in IR:\n%s", ir)
	}
	if !strings.Contains(ir, "llvm.lifetime.end") {
		t.Fatalf("missing lifetime.end in IR:\n%s", ir)
	}
}
