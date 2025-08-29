package lexbridge

import (
  "strings"
  "testing"
)

func TestToGoTokens_Basic(t *testing.T) {
  // Minimal NDJSON covering KW + punctuation + literals
  raw := `{"kind":"KW","text":"def","line":1,"col":1}
{"kind":"IDENT","text":"main","line":1,"col":5}
{"kind":"LPAREN","text":"(","line":1,"col":9}
{"kind":"RPAREN","text":")","line":1,"col":10}
{"kind":"ARROW","text":"->","line":1,"col":12}
{"kind":"IDENT","text":"int","line":1,"col":15}
{"kind":"COLON","text":":","line":1,"col":18}
{"kind":"NEWLINE","text":"","line":1,"col":19}
{"kind":"INDENT","text":"","line":2,"col":1}
{"kind":"KW","text":"return","line":2,"col":3}
{"kind":"INT","text":"0","line":2,"col":10}
{"kind":"NEWLINE","text":"","line":2,"col":11}
{"kind":"DEDENT","text":"","line":3,"col":1}
{"kind":"EOF","text":"","line":3,"col":1}
`
  toks, err := ParseNDJSON(strings.NewReader(raw))
  if err != nil {
    t.Fatalf("ParseNDJSON err: %v", err)
  }
  got, err := ToGoTokens(toks)
  if err != nil {
    t.Fatalf("ToGoTokens err: %v", err)
  }
  // smoke: last must be EOF
  if got[len(got)-1].Kind.String() != "EOF" {
    t.Fatalf("want trailing EOF, got %v", got[len(got)-1].Kind)
  }
  // spot-check a couple
  if g := got[0]; g.Kind.String() != "def" {
    t.Fatalf("got[0].Kind=%s want def", g.Kind.String())
  }
  if g := got[1]; g.Kind.String() != "IDENT" || g.Lex != "main" {
    t.Fatalf("bad IDENT: %#v", g)
  }
}

func TestMapToGoKind_Unmapped(t *testing.T) {
  if _, ok := MapToGoKind("KW", "nonexistent_kw"); ok {
    t.Fatalf("unexpected ok for nonexistent keyword")
  }
  if _, ok := MapToGoKind("TOTALLY_UNKNOWN_KIND", ""); ok {
    t.Fatalf("unexpected ok for unknown kind")
  }
}
