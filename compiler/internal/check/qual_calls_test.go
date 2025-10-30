package check

import (
  "testing"

  "github.com/desilang/desi/compiler/internal/parse"
  "github.com/desilang/desi/compiler/internal/resolve"
  "github.com/desilang/desi/compiler/internal/types"
)

func TestQualifiedCall_Ok(t *testing.T) {
  ldr := resolve.NewMemLoader(map[string]string{
    "math/__mod.desi": `
pub def add(a: int, b: int) -> int:
  return a + b
`,
  })
  mod, diags := parse.ParseFile("main.desi", []byte(`
import math

def main() -> int:
	print(math.add(2, 3))
	return 0
`))
  if len(diags) != 0 {
    t.Fatalf("unexpected parse diags: %+v", diags)
  }
  res := CheckWithLoader(mod, ldr)
  mustNoDiags(t, res.Diags)

  // sanity: ensure types package is linked (not strictly needed)
  _ = types.Int
}

func TestQualifiedCall_BadMember_DME0003(t *testing.T) {
  ldr := resolve.NewMemLoader(map[string]string{
    "math/__mod.desi": `
pub def add(a: int, b: int) -> int:
  return a + b

def sqrt(x: int) -> int:
  return x
`,
  })
  mod, diags := parse.ParseFile("main.desi", []byte(`
import math

def main() -> int:
	print(math.sqrt(9))
	return 0
`))
  if len(diags) != 0 {
    t.Fatalf("unexpected parse diags: %+v", diags)
  }
  res := CheckWithLoader(mod, ldr)

  var got int
  for _, d := range res.Diags {
    if d.CodeID == "DME0003" && d.Message == "math has no exported 'sqrt'" {
      got++
    }
  }
  if got != 1 {
    t.Fatalf("expected 1 DME0003 for missing export, got %d (diags=%+v)", got, res.Diags)
  }
}

func TestQualifiedCall_Alias_Ok(t *testing.T) {
  ldr := resolve.NewMemLoader(map[string]string{
    "math/__mod.desi": `
pub def add(a: int, b: int) -> int:
  return a + b
`,
  })
  mod, diags := parse.ParseFile("main.desi", []byte(`
import math as M

def main() -> int:
	print(M.add(2, 3))
	return 0
`))
  if len(diags) != 0 {
    t.Fatalf("unexpected parse diags: %+v", diags)
  }
  res := CheckWithLoader(mod, ldr)
  mustNoDiags(t, res.Diags)
}

func TestQualifiedCall_CountsAsImportUse_NoDMW0004(t *testing.T) {
  ldr := resolve.NewMemLoader(map[string]string{
    "math/__mod.desi": `
pub def add(a: int, b: int) -> int:
  return a + b
`,
  })
  mod, diags := parse.ParseFile("main.desi", []byte(`
import math

def main() -> int:
	math.add(1, 2)
	return 0
`))
  if len(diags) != 0 {
    t.Fatalf("unexpected parse diags: %+v", diags)
  }
  res := CheckWithLoader(mod, ldr)

  for _, d := range res.Diags {
    if d.CodeID == "DMW0004" {
      t.Fatalf("did not expect DMW0004 (unused import) when using module qualifier; diags=%+v", res.Diags)
    }
  }
}
