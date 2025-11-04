package hir_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/hir"
)

func TestPrint_AsyncNodes_Smoke(t *testing.T) {
	fn := &hir.Func{
		Name: "async_demo",
		Blocks: []*hir.Block{{
			Name: "entry",
			Stmts: []hir.Stmt{
				&hir.FutureNew{Dst: hir.Temp{Name: "%t1"}},
				&hir.Await{Fut: hir.Temp{Name: "%t1"}, Dst: hir.Temp{Name: "%t2"}},
				&hir.FutureComplete{Fut: hir.Temp{Name: "%t1"}, Val: hir.ConstInt{Text: "42"}},
				&hir.Ret{},
			},
		}},
	}

	var buf bytes.Buffer
	hir.Print(&buf, fn)
	out := buf.String()

	wantLines := []string{
		"func async_demo",
		"  block entry",
		"    %t1 = future.new",
		"    await %t1 -> %t2",
		"    future.complete %t1, 42",
		"    ret",
	}
	for _, w := range wantLines {
		if !strings.Contains(out, w) {
			t.Fatalf("missing expected line %q in:\n%s", w, out)
		}
	}
}
