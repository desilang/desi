package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
)

func TestM6P2_FromImport_ModesFlow_InoutRequiresLvalue(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"utilx/__mod.desi": `
pub def tweak(inout x:int):
	return none
`,
	})
	mod, diags := parse.ParseFile("main.desi", []byte(`
from utilx import tweak

def main():
	tweak(1)
	return none
`))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	res := CheckWithLoader(mod, ldr)
	mustHaveSomeDiagContaining(t, res.Diags, "mutable lvalue")
}

func TestM6P2_QualifiedCall_ModesFlow_Aliasing(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"utilx/__mod.desi": `
pub def touch(inout a:int, ref b:int):
	return none
`,
	})
	mod, diags := parse.ParseFile("main.desi", []byte(`
import utilx

def main():
	let x = 0
	utilx.touch(x, x)
	return none
`))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	res := CheckWithLoader(mod, ldr)
	mustHaveSomeDiagContaining(t, res.Diags, "alias")
}
