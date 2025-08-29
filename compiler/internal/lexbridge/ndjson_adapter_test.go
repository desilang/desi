package lexbridge

import (
	"strings"
	"testing"
)

func TestParseNDJSON(t *testing.T) {
	raw := `{"kind":"KW","text":"def","line":1,"col":1}
{"kind":"IDENT","text":"main","line":1,"col":5}
{"kind":"LPAREN","text":"(","line":1,"col":9}
{"kind":"RPAREN","text":")","line":1,"col":10}
{"kind":"ARROW","text":"->","line":1,"col":12}
{"kind":"IDENT","text":"int","line":1,"col":15}
{"kind":"COLON","text":":","line":1,"col":18}
{"kind":"NEWLINE","text":"","line":1,"col":19}
{"kind":"INDENT","text":"","line":2,"col":1}
{"kind":"STR","text":"hi","line":2,"col":3}
{"kind":"EOF","text":"","line":2,"col":7}
`
	toks, err := ParseNDJSON(strings.NewReader(raw))
	if err != nil {
		// Should still parse successfully; the error is only returned for malformed lines
		// which we don't have here.
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toks) != 11 {
		t.Fatalf("got %d tokens, want 11", len(toks))
	}
	if toks[0].Kind != "KW" || toks[0].Text != "def" || toks[0].Line != 1 || toks[0].Col != 1 {
		t.Fatalf("bad first token: %#v", toks[0])
	}

	// smoke test the formatter
	out := DebugFormat(toks, 3)
	if !strings.Contains(out, "1:1  KW") {
		t.Fatalf("unexpected DebugFormat: %s", out)
	}
}
