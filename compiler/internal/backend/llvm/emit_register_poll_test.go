package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestM8E_Emit_RegisterPoll(void *testing.T) {
	f := &hir.Func{
		Name: "main",
		Blocks: []*hir.Block{
			{
				Name: "entry",
				Stmts: []hir.Stmt{
					&hir.Let{Name: "fut", Init: hir.ConstInt{Text: "0"}}, // just to have a %fut
					&hir.Call{
						Fn:   "__future_register_poll",
						Args: []hir.Value{hir.Var{Name: "fut"}, hir.Var{Name: "&foo$poll"}, hir.Var{Name: "&frame"}},
					},
					&hir.Ret{},
				},
			},
		},
	}
	m := llvm.NewModule("test")
	m.EmitFunc(f)
	ir := m.IR()

	if !strings.Contains(ir, "declare void @__future_register_poll(ptr, ptr, ptr)") {
		t.Fatalf("missing declare for __future_register_poll in IR:\n%s", ir)
	}
	if !strings.Contains(ir, "call void @__future_register_poll(") {
		t.Fatalf("missing call to __future_register_poll in IR:\n%s", ir)
	}
}
