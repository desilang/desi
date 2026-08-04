package llvm

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/hir"
)

// fn builds a function whose single block holds the given statements.
func fn(name string, stmts ...hir.Stmt) *hir.Func {
	return &hir.Func{
		Name:   name,
		Blocks: []*hir.Block{{Name: "entry", Stmts: stmts}},
	}
}

func TestGuardExemptOnlyCallFreeFunctions(t *testing.T) {
	funcs := []*hir.Func{
		// Calls nothing: cannot be re-entered, so it needs no guard.
		fn("leaf",
			&hir.BinaryOp{Op: "+", Dst: hir.Temp{Name: "%t0"}, Type: "i32"},
			&hir.Ret{},
		),
		// Calls itself.
		fn("selfRecursive",
			&hir.Call{Fn: "selfRecursive"},
			&hir.Ret{},
		),
		// Calls something else, so it is not call-free even though the callee
		// happens to be a leaf. The rule is deliberately this narrow.
		fn("callsLeaf",
			&hir.Call{Fn: "leaf"},
			&hir.Ret{},
		),
		// Only calls the runtime. Still not call-free: a runtime function can
		// invoke a function pointer it was handed earlier, and this rule does
		// not track where those travel.
		fn("callsRuntime",
			&hir.Call{Fn: "print_item"},
			&hir.Ret{},
		),
		// A tail call is a call.
		fn("tailRecursive",
			&hir.TailCall{Fn: "tailRecursive"},
		),
		// Mutual recursion: neither calls itself, both must keep a guard.
		fn("mutualA", &hir.Call{Fn: "mutualB"}, &hir.Ret{}),
		fn("mutualB", &hir.Call{Fn: "mutualA"}, &hir.Ret{}),
	}

	got := GuardExemptFunctions(funcs)

	want := map[string]bool{"leaf": true}
	for _, f := range funcs {
		if got[f.Name] != want[f.Name] {
			t.Errorf("%s: exempt=%v, want %v", f.Name, got[f.Name], want[f.Name])
		}
	}
}

// A call anywhere in the function counts, not just in the entry block — a
// function whose only call sits in a loop body is still not call-free.
func TestGuardExemptScansEveryBlock(t *testing.T) {
	f := &hir.Func{
		Name: "callsInLaterBlock",
		Blocks: []*hir.Block{
			{Name: "entry", Stmts: []hir.Stmt{&hir.Jump{}}},
			{Name: "body", Stmts: []hir.Stmt{&hir.Call{Fn: "somewhere"}}},
			{Name: "exit", Stmts: []hir.Stmt{&hir.Ret{}}},
		},
	}
	if GuardExemptFunctions([]*hir.Func{f})["callsInLaterBlock"] {
		t.Fatal("a function whose only call is outside the entry block was exempted")
	}
}

func TestGuardExemptToleratesEmptyAndNilBlocks(t *testing.T) {
	funcs := []*hir.Func{
		{Name: "noBlocks"},
		{Name: "nilBlock", Blocks: []*hir.Block{nil}},
		nil,
	}
	got := GuardExemptFunctions(funcs)
	// A function with no body makes no calls, so it is exempt — and more to the
	// point, this must not panic.
	if !got["noBlocks"] || !got["nilBlock"] {
		t.Fatalf("expected empty functions to be exempt, got %v", got)
	}
}
