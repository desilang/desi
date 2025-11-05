package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestSSAOnlyLet_OmitsAllocaAndLifetimes(t *testing.T) {
	m := llvm.NewModule("ssa_lets")
	fn := &hir.Func{
		Name: "ssa_let_demo",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Let{Name: "x", Init: hir.ConstInt{Text: "7"}},
				&hir.Ret{Val: hir.Var{Name: "x"}}, // should fold to ret i32 7 via SSA alias
			},
		}},
	}
	m.EmitFunc(fn)
	ir := m.IR()

	if strings.Contains(ir, "alloca %x") || strings.Contains(ir, " %x = alloca ") {
		t.Fatalf("expected no alloca for SSA-only let, got:\n%s", ir)
	}
	if strings.Contains(ir, "llvm.lifetime.start") {
		t.Fatalf("expected no lifetime.start for SSA-only let, got:\n%s", ir)
	}
	if strings.Contains(ir, "llvm.lifetime.end") {
		t.Fatalf("expected no lifetime.end for SSA-only let, got:\n%s", ir)
	}
	if !strings.Contains(ir, "ret i32 7") {
		t.Fatalf("expected ret i32 7 via SSA alias, got:\n%s", ir)
	}
}
