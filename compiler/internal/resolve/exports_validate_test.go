package resolve

import (
  "testing"

  "github.com/desilang/desi/compiler/internal/parse"
)

func TestValidateFromItemExports_MissingAndPresent(t *testing.T) {
  fs := map[string]string{
    "math/__mod.desi": `
def add(a: int, b: int) -> int:
  return a + b

def helper(a):
  return a

def area(r: float) -> float:
  return r
`,
  }
  ldr := NewMemLoader(fs)

  // Case 1: valid import of a typed function
  srcOK := `from math import add`
  modOK, diags := parse.ParseFile("main_ok.desi", []byte(srcOK))
  if len(diags) != 0 {
    t.Fatalf("unexpected parse diags (ok): %+v", diags)
  }
  got := ValidateFromItemExports(modOK, ldr)
  if len(got) != 0 {
    t.Fatalf("expected no diags for valid import, got %d: %+v", len(got), got)
  }

  // Case 2: invalid import of untyped/private function -> DME0003
  srcBad := `from math import helper`
  modBad, diags := parse.ParseFile("main_bad.desi", []byte(srcBad))
  if len(diags) != 0 {
    t.Fatalf("unexpected parse diags (bad): %+v", diags)
  }
  got = ValidateFromItemExports(modBad, ldr)
  if len(got) != 1 {
    t.Fatalf("expected 1 diag, got %d: %+v", len(got), got)
  }
  if got[0].CodeID != "DME0003" {
    t.Fatalf("expected DME0003, got %s", got[0].CodeID)
  }
  if got[0].Message == "" || got[0].Primary.Span.File == "" {
    t.Fatalf("expected helpful message and primary span")
  }
}
