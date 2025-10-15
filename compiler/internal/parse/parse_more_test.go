package parse

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func render2(n ast.Node) string {
	var b bytes.Buffer
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

func TestAsyncDefPrint(t *testing.T) {
	src := "async def ping(x: int):\n\treturn x\n"
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := render2(mod)
	want := strings.TrimSpace(`
Module("<mem>")
	Func async ping(x: int)
		Block
			Return Ident(x)
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestEOFActsAsNewline(t *testing.T) {
	// No trailing newline after the last statement.
	src := "def f():\n\tlet x = 1"
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := render2(mod)
	want := strings.TrimSpace(`
Module("<mem>")
	Func f()
		Block
			Let x = Int(1)
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestCommentBeforeIndentOK(t *testing.T) {
	// Ensure blank/comment lines before the first indented stmt are accepted.
	src := "def f():\n\t# leading comment\n\treturn 1\n"
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := render2(mod)
	want := strings.TrimSpace(`
Module("<mem>")
	Func f()
		Block
			Return Int(1)
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPostfixGreedyAnyOrder(t *testing.T) {
	// foo(1)[i].bar(2)[j]  -> greedy chain in mixed order
	src := "def z():\n\tlet a = foo(1)[i].bar(2)[j]\n"
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := render2(mod)
	// Outer to inner: Index(Call(Field(Index(Call(Ident(foo))), .bar(2))), [j])
	want := strings.TrimSpace(`
Module("<mem>")
	Func z()
		Block
			Let a = Index Call Field Index Call Ident(foo)(Int(1))[Ident(i)].bar(Int(2))[Ident(j)]
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
