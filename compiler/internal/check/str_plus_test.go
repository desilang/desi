package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
)

func TestStrPlusImplicit_OK(t *testing.T) {
	src := []byte(`
def main() -> int:
  print("answer=" + 42)
  print(3.14 + "π")
  print(true + " is truthy")
  return 0
`)
	mod, pdiags := parse.ParseFile("<mem>", src)
	if len(pdiags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", pdiags)
	}
	res := CheckWithLoader(mod, resolve.NewMemLoader(nil))
	mustNoDiags(t, res.Diags)
}
