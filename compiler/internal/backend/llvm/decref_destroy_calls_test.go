package llvm_test

import (
  "strings"
  "testing"

  "github.com/desilang/desi/compiler/internal/backend/llvm"
  "github.com/desilang/desi/compiler/internal/hir"
)

func TestEmit_DeclAndCall_DecRef(t *testing.T) {
  fn := &hir.Func{
    Name: "f",
    Blocks: []*hir.Block{{
      Name: "entry",
      Stmts: []hir.Stmt{
        &hir.Let{Name: "x", Init: hir.ConstInt{Text: "1"}},
        &hir.DecRef{Val: hir.Var{Name: "x"}},
        &hir.Ret{Val: hir.ConstInt{Text: "0"}},
      },
    }},
  }
  m := llvm.NewModule("test")
  m.EmitFunc(fn)
  ir := m.IR()

  if !strings.Contains(ir, "declare void @__rc_dec(ptr)") {
    t.Fatalf("missing declaration for __rc_dec:\n%s", ir)
  }
  if !strings.Contains(ir, "call void @__rc_dec(ptr %x)") {
    t.Fatalf("missing call to __rc_dec(ptr %x):\n%s", ir)
  }
}

func TestEmit_DeclAndCall_DestroyArena(t *testing.T) {
  fn := &hir.Func{
    Name: "g",
    Blocks: []*hir.Block{{
      Name: "entry",
      Stmts: []hir.Stmt{
        &hir.Let{Name: "arena", Init: nil},
        &hir.DestroyArena{Arena: hir.Var{Name: "arena"}},
        &hir.Ret{Val: hir.ConstInt{Text: "0"}},
      },
    }},
  }
  m := llvm.NewModule("test")
  m.EmitFunc(fn)
  ir := m.IR()

  if !strings.Contains(ir, "declare void @__arena_destroy(ptr)") {
    t.Fatalf("missing declaration for __arena_destroy:\n%s", ir)
  }
  if !strings.Contains(ir, "call void @__arena_destroy(ptr %arena)") {
    t.Fatalf("missing call to __arena_destroy(ptr %arena):\n%s", ir)
  }
}
