package lexer

import "testing"

func kindsFrom(src string) []TokKind {
  l := New(src)
  var kinds []TokKind
  for {
    t := l.Next()
    kinds = append(kinds, t.Kind)
    if t.Kind == TokEOF {
      break
    }
  }
  return kinds
}

func TestStubEOF(t *testing.T) {
  ks := kindsFrom("")
  if got, want := ks[len(ks)-1], TokEOF; got != want {
    t.Fatalf("expected EOF, got %v", got)
  }
}

func TestLetAssignAndNewlines(t *testing.T) {
  src := "let mut y = 0\ny := y + 1\n"
  ks := kindsFrom(src)
  want := []TokKind{
    TokLet, TokMut, TokIdent, TokEq, TokInt, TokNewline,
    TokIdent, TokAssign, TokIdent, TokPlus, TokInt, TokNewline,
    TokEOF,
  }
  if len(ks) != len(want) {
    t.Fatalf("token count mismatch: got %d, want %d (%v)", len(ks), len(want), ks)
  }
  for i := range want {
    if ks[i] != want[i] {
      t.Fatalf("ks[%d]=%v, want %v (full=%v)", i, ks[i], want[i], ks)
    }
  }
}

func TestIndentDedent(t *testing.T) {
  src := "" +
    "def f(a: i32) -> i32:\n" +
    "  let x = 1\n" +
    "  return x\n"
  ks := kindsFrom(src)
  want := []TokKind{
    TokDef, TokIdent, TokLParen, TokIdent, TokColon, TokIdent, TokRParen, TokArrow, TokIdent, TokColon, TokNewline,
    TokIndent,
    TokLet, TokIdent, TokEq, TokInt, TokNewline,
    TokReturn, TokIdent, TokNewline,
    TokDedent,
    TokEOF,
  }
  if len(ks) != len(want) {
    t.Fatalf("token count mismatch: got %d, want %d (%v)", len(ks), len(want), ks)
  }
  for i := range want {
    if ks[i] != want[i] {
      t.Fatalf("ks[%d]=%v, want %v (full=%v)", i, ks[i], want[i], ks)
    }
  }
}
