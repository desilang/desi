package parser_test

import (
  "testing"

  "github.com/desilang/desi/compiler/internal/parser"
)

// Each case is wrapped in a tiny function so we don't depend on whether
// top-level let/assign is currently allowed by the grammar.
// We only assert successful parse here; typechecking/emit are covered elsewhere.
func TestMultiVarLetAndAssignParse(t *testing.T) {
  type tc struct {
    name string
    body string // body of main()
  }
  cases := []tc{
    {
      name: "let_multi_simple",
      body: "let a, b, c = 1, 2, 3",
    },
    {
      name: "let_multi_mut",
      body: "let mut a, b, c = 1, 2, 3",
    },
    {
      name: "short_assign_multi",
      body: "a, b := 4, 5",
    },
    {
      name: "let_multi_with_annotations_tuple",
      body: "let (a:int, b, c:str) = 1, \"x\", \"y\"",
    },
  }

  for _, c := range cases {
    t.Run(c.name, func(t *testing.T) {
      src := "def main() -> int:\n" +
        "  " + c.body + "\n" +
        "  return 0\n"
      p := parser.New(src)
      if _, err := p.ParseFile(); err != nil {
        t.Fatalf("parse failed for %s: %v\nsource:\n%s", c.name, err, src)
      }
    })
  }
}
