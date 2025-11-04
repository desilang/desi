package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

// Ensure we never print lifetime.end after a ret in the same block.
func TestNoLifetimeEndAfterRet(t *testing.T) {
	m := llvm.NewModule("lifetimes")
	fn := &hir.Func{
		Name: "lifet_after_ret",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Let{Name: "x"}, // stack local (will lifetime.start)
				&hir.Assign{LHS: "x", RHS: hir.ConstInt{Text: "1"}}, // mutate
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},              // unconditional ret
				// (any further stmts ignored for lifetime end purposes)
			},
		}},
	}
	m.EmitFunc(fn)
	ir := m.IR()

	if !strings.Contains(ir, "alloca %x") && !strings.Contains(ir, "%x = alloca") {
		t.Fatalf("expected an alloca for non-SSA let, got:\n%s", ir)
	}
	if !strings.Contains(ir, "llvm.lifetime.start") {
		t.Fatalf("expected lifetime.start for non-SSA let, got:\n%s", ir)
	}
	if strings.Contains(ir, "llvm.lifetime.end") {
		t.Fatalf("did NOT expect lifetime.end after ret, got:\n%s", ir)
	}
}
