package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

// Ensure we never print lifetime.end *after* a ret in the same block.
// It's OK (and preferred) to close lifetimes immediately *before* ret.
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
			},
		}},
	}
	m.EmitFunc(fn)
	ir := m.IR()

	if !strings.Contains(ir, "alloca") {
		t.Fatalf("expected an alloca for non-SSA let, got:\n%s", ir)
	}
	if !strings.Contains(ir, "llvm.lifetime.start") {
		t.Fatalf("expected lifetime.start for non-SSA let, got:\n%s", ir)
	}
	// ret must be present
	retIdx := strings.Index(ir, "\n  ret ")
	if retIdx < 0 {
		t.Fatalf("expected a ret in IR, got:\n%s", ir)
	}

	// If there are any lifetime.end calls, the last one must be BEFORE ret.
	lastEnd := strings.LastIndex(ir, "llvm.lifetime.end")
	if lastEnd >= 0 && lastEnd > retIdx {
		t.Fatalf("did NOT expect lifetime.end after ret, got:\n%s", ir)
	}
}
