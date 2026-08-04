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

// Appending stores inline while there is room, and calls the runtime to grow.
//
// The type tag has to be maintained here rather than left to the runtime: it
// starts at 0 and gets promoted the first time a non-int arrives, and a list
// whose tag never got promoted prints its elements wrongly. It is written
// through a select rather than behind a branch, so the check costs a store on a
// line the length update already dirtied instead of a mispredict.
func TestListAppendEmitsInlineStoreAndKeepsGrowCall(t *testing.T) {
	fn := &hir.Func{
		Name: "f",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.Call{
					Fn: "list_append",
					Args: []hir.Value{
						hir.Temp{Name: "%lst"},
						hir.Temp{Name: "%item"},
						hir.ConstInt{Text: "0", Type: "i32"},
					},
					Type: "void",
				},
				&hir.Ret{Val: hir.ConstInt{Text: "0"}},
			},
		}},
	}
	m := llvm.NewModule("test")
	m.EmitFunc(fn)
	ir := m.IR()

	for _, want := range []string{
		"getelementptr inbounds i8, ptr %lst, i64 8",  // length
		"getelementptr inbounds i8, ptr %lst, i64 16", // capacity
		"icmp ult i64",                                // room to store?
		"getelementptr inbounds i8, ptr %lst, i64 24", // type tag
		"select i1",                                   // tag maintained branch-free
		"store ptr %item",                             // the element itself
		"add i64",                                     // length + 1
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("inline append is missing %q:\n%s", want, ir)
		}
	}

	if !strings.Contains(ir, "call void @list_append(") {
		t.Errorf("the grow path no longer calls the runtime; reallocation and the "+
			"arena branch would have to be duplicated inline:\n%s", ir)
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
