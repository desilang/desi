package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestM8H_Emit_PollParamSignature(t *testing.T) {
	f := &hir.Func{
		Name:   "foo$poll",
		Params: []hir.Param{{Name: "frame"}},
		Blocks: []*hir.Block{
			{Name: "entry"},
		},
	}

	m := llvm.NewModule("test")
	m.EmitFunc(f)
	ir := m.IR()

	if !strings.Contains(ir, "define i32 @foo$poll(ptr %frame)") {
		t.Fatalf("missing poll param in LLVM signature:\n%s", ir)
	}
}
