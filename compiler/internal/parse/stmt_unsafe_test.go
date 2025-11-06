package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// Positive: happy path parses and pretty-prints an Unsafe block.
func TestUnsafe_Parse_Basic(t *testing.T) {
	src := `
def main() -> int:
	unsafe:
		let x = 1
	0
`
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	var b strings.Builder
	ast.Print(&b, mod)
	got := strings.TrimSpace(b.String())
	want := strings.TrimSpace(`
Module("<mem>")
	Func main() -> int
		Block
			Unsafe
				Block
					Let x = Int(1)
			Int(0)
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// Negative: missing ":" after 'unsafe'
func TestUnsafe_MissingColon_Diag(t *testing.T) {
	src := "def f():\n\tunsafe\n\t\t0\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) == 0 {
		t.Fatalf("expected a diagnostic for missing ':'")
	}
	found := false
	for _, d := range diags {
		if d.CodeID == "DPE0002" { // expected a different token
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DPE0002 among diagnostics, got: %+v", diags)
	}
}

// Negative: missing indented block after 'unsafe:'
func TestUnsafe_MissingIndentedBlock_Diag(t *testing.T) {
	src := "def f():\n\tunsafe:\n\t0\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for missing indented block")
	}
}
