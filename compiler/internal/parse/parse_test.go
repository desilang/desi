package parse

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func render(n ast.Node) string {
	var buf bytes.Buffer
	ast.Print(&buf, n)
	return strings.TrimSpace(buf.String())
}

func TestPowAndPipeline(t *testing.T) {
	src := "def powpipe(a: int, b: int) -> int:\n\tlet x = a ** 2 |> b + 1\n\treturn x\n"
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := render(mod)
	want := strings.TrimSpace(`
Module("<mem>")
	Func powpipe(a: int, b: int) -> int
		Block
			Let x = ((Ident(a) ** Int(2)) |> (Ident(b) + Int(1)))
			Return Ident(x)
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPostfixChain_CallIndexField(t *testing.T) {
	src := "def chains():\n\tlet z = foo(1, 2)[x].y\n"
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := render(mod)
	want := strings.TrimSpace(`
Module("<mem>")
	Func chains()
		Block
			Let z = Field Index Call Ident(foo)(Int(1), Int(2))[Ident(x)].y
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestUnclosedParen_YieldsDPE0003(t *testing.T) {
	// Missing ')' after params.
	src := "def f(a: int:\n\treturn a\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) == 0 {
		t.Fatalf("expected at least one diagnostic")
	}
	found := false
	for _, d := range diags {
		if d.CodeID == "DPE0003" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DPE0003 (unclosed delimiter), got %+v", diags)
	}
}
