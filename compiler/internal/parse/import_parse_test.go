package parse

import (
  "strings"
  "testing"
)

func TestImport_ParseAndPrint(t *testing.T) {
  src := `
import util.math
import std.io as io
from util.math import add, sub as minus
`
  mod, diags := ParseFile("<mem>", []byte(src))
  if len(diags) != 0 {
    t.Fatalf("unexpected diags: %+v", diags)
  }
  out := render(mod)

  wantFrags := []string{
    `Import util.math`,
    `Import std.io as io`,
    `From util.math import add, sub as minus`,
  }
  for _, w := range wantFrags {
    if !strings.Contains(out, w) {
      t.Fatalf("AST render missing substring:\nwant contains: %s\n--- got ---\n%s", w, out)
    }
  }
}
