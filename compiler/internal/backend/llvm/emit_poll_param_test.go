package llvm_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestEmit_PollHasFrameParam(t *testing.T) {
	m := llvm.NewModule("m")
	fn := &hir.Func{
		Name:   "foo$poll",
		Params: []hir.Param{{Name: "frame"}},
		Blocks: []*hir.Block{{Name: "entry"}},
	}
	m.EmitFunc(fn)
	ir := m.IR()
	if !strings.Contains(ir, "define i32 @foo$poll(ptr %frame)") {
		t.Fatalf("expected poll param in LLVM define, got:\n%s", ir)
	}
}
