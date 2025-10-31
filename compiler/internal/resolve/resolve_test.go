package resolve

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parse"
)

func mustParse(t *testing.T, name, src string) *ast.Module {
	t.Helper()
	mod, diags := parse.ParseFile(name, []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags for %s: %+v", name, diags)
	}
	return mod
}

func TestResolve_BindsImportsAndFromItems(t *testing.T) {
	// Std-less library: top-level "math" (typed+pub) and "io"
	ldr := NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def add(x: int, y: int) -> int:
  return x + y

pub def sub(x: int, y: int) -> int:
  return x - y
`,
		"io.desi": `def println(x): return 0`,
	})

	mainSrc := `
import math
import io
from math import add, sub as minus
`
	main := mustParse(t, "main.desi", mainSrc)

	diags, info := Resolve(main, ldr)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	// Imports: math -> "math"; io -> "io"
	if _, ok := info.Imports["math"]; !ok {
		t.Fatalf("missing import binding ''math'': %#v", info.Imports)
	}
	if _, ok := info.Imports["io"]; !ok {
		t.Fatalf("missing import binding ''io'': %#v", info.Imports)
	}

	// From-items: add, minus
	if _, ok := info.FromItems["add"]; !ok {
		t.Fatalf("missing from-item binding ''add'': %#v", info.FromItems)
	}
	if _, ok := info.FromItems["minus"]; !ok {
		t.Fatalf("missing from-item binding ''minus'': %#v", info.FromItems)
	}

	// Graph edges: simple sanity check
	var sb strings.Builder
	for from, tos := range info.Graph.edges {
		_, _ = sb.WriteString(from)
		_, _ = sb.WriteString(":")
		for _, to := range tos {
			_, _ = sb.WriteString("->")
			_, _ = sb.WriteString(to)
		}
		_, _ = sb.WriteString("\n")
	}
	got := sb.String()
	var mainLine string
	for _, ln := range strings.Split(got, "\n") {
		if strings.HasPrefix(ln, "main.desi:") {
			mainLine = ln
			break
		}
	}
	if mainLine == "" {
		t.Fatalf("no graph line for main.desi; got:\n%s", got)
	}
	if !strings.Contains(mainLine, "->math") {
		t.Fatalf("graph missing edge to math:\n%s", got)
	}
	if !strings.Contains(mainLine, "->io") {
		t.Fatalf("graph missing edge to io:\n%s", got)
	}
}
