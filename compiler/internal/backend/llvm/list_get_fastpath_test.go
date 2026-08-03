package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

// Reading a list element emits the check and the load inline, and calls the
// runtime only when the check fails.
//
// The shape matters as much as the presence. Inlining list_get's whole body is
// measurably *slower* than calling it, because the body ends in an fprintf on
// the failure path and that gets dragged into the loop. What pays is the split:
// a small hot path, and a branch away to something out of line. So this test
// pins both halves -- an inline load AND a surviving call.
func TestListGetEmitsInlineFastPathAndKeepsSlowCall(t *testing.T) {
	fn := &hir.Func{
		Name: "f",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Call{
					Dst:  hir.Temp{Name: "%e"},
					Fn:   "list_get",
					Args: []hir.Value{hir.Temp{Name: "%lst"}, hir.ConstInt{Text: "3", Type: "i64"}},
					Type: "ptr",
				},
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},
			},
		}},
	}
	m := llvm.NewModule("test")
	m.EmitFunc(fn)
	ir := m.IR()

	// The hot path: bounds test against the length field, then a direct load.
	for _, want := range []string{
		"icmp ne ptr",             // null check
		"getelementptr inbounds i8, ptr %lst, i64 8", // length lives at offset 8
		"icmp slt i64",            // upper bound
		"load ptr, ptr %lst",      // data lives at offset 0
		"getelementptr inbounds ptr",
		"phi ptr",
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("inline fast path is missing %q:\n%s", want, ir)
		}
	}

	// The cold path stays a call, so the error message and the negative-index
	// rule keep one implementation and stay out of the loop body.
	if !strings.Contains(ir, "call ptr @list_get(") {
		t.Errorf("the slow path no longer calls the runtime; the error handling "+
			"would have to be duplicated inline:\n%s", ir)
	}
}

// A list_get whose result is unused by name must not take the fast path, since
// there would be no destination for the phi to define.
func TestListGetWithoutDestinationFallsBackToACall(t *testing.T) {
	fn := &hir.Func{
		Name: "g",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Call{
					Fn:   "list_get",
					Args: []hir.Value{hir.Temp{Name: "%lst"}, hir.ConstInt{Text: "0", Type: "i64"}},
					Type: "ptr",
				},
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},
			},
		}},
	}
	m := llvm.NewModule("test")
	m.EmitFunc(fn)
	if ir := m.IR(); strings.Contains(ir, "phi ptr") {
		t.Errorf("emitted a phi with no destination to bind it to:\n%s", ir)
	}
}
