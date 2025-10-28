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
	ldr := NewMemLoader(map[string]string{
		// packages
		"util/__mod.desi":      `def id(x): return x`,
		"util/math/__mod.desi": `def add(x,y): return x+y`,
		// leaf file module as final segment
		"std/io.desi": `def println(x): return 0`,
	})

	mainSrc := `
import util.math
import std.io as io
from util.math import add, sub as minus
`
	main := mustParse(t, "main.desi", mainSrc)

	diags, info := Resolve(main, ldr)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	// Imports: util.math -> local "math"; std.io as io -> local "io"
	if _, ok := info.Imports["math"]; !ok {
		t.Fatalf("missing import binding 'math': %#v", info.Imports)
	}
	if _, ok := info.Imports["io"]; !ok {
		t.Fatalf("missing import binding 'io': %#v", info.Imports)
	}

	// From-items: add, minus
	if _, ok := info.FromItems["add"]; !ok {
		t.Fatalf("missing from-item binding 'add': %#v", info.FromItems)
	}
	if _, ok := info.FromItems["minus"]; !ok {
		t.Fatalf("missing from-item binding 'minus': %#v", info.FromItems)
	}

	// Graph should at least have edges to imported modules.
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
	if !strings.Contains(got, `main.desi->util.math`) {
		t.Fatalf("graph missing edge to util.math:\n%s", got)
	}
	if !strings.Contains(got, `main.desi->std.io`) {
		t.Fatalf("graph missing edge to std.io:\n%s", got)
	}
}
