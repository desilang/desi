package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
)

func TestUnusedImport_WarnsDMW0004(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"io.desi": `def println(x): return 0`,
	})

	mod, diags := parse.ParseFile("main.desi", []byte(`import io`))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	res := CheckWithLoader(mod, ldr)

	var got int
	for _, d := range res.Diags {
		if d.CodeID == "DMW0004" {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("expected 1 DMW0004, got %d (diags=%+v)", got, res.Diags)
	}
}

func TestUnusedFromItem_WarnsDMW0005(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def add(a: int, b: int) -> int:
  return a + b
`,
	})

	mod, diags := parse.ParseFile("main.desi", []byte(`from math import add`))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	res := CheckWithLoader(mod, ldr)

	var got int
	for _, d := range res.Diags {
		if d.CodeID == "DMW0005" {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("expected 1 DMW0005, got %d (diags=%+v)", got, res.Diags)
	}
}
