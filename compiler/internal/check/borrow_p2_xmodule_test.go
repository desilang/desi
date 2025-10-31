package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
)

func TestM6P2_FromImport_ModesFlow_InoutRequiresLvalue(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"mut/__mod.desi": `
pub def tweak(inout x:int) -> none:
  return none
`,
	})
	mod, diags := parse.ParseFile("main.desi", []byte(`
from mut import tweak

def main() -> none:
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
		"mut/__mod.desi": `
pub def touch(inout a:int, ref b:int) -> none:
  return none
`,
	})
	mod, diags := parse.ParseFile("main.desi", []byte(`
import mut

def main() -> none:
  let x = 0
  mut.touch(x, x)
  return none
`))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	res := CheckWithLoader(mod, ldr)
	mustHaveSomeDiagContaining(t, res.Diags, "alias")
}
