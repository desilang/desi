package parse

import (
	"strings"
	"testing"
)

func TestDiagnostics_LambdaHeadAndCompHead(t *testing.T) {
	src := `
let badLam = (f(x)) => x   # invalid lambda head before =>
let a = [x]                # missing 'for'
let b = {k: v}             # VALID dict literal now (not an error!)
let c = #{x}               # missing 'for'
`
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) < 3 {
		t.Fatalf("expected at least 3 diagnostics, got %d", len(diags))
	}
	// We don't assert strict ordering; just that the expected messages appear.
	var lam, list, set bool
	for _, d := range diags {
		if d.Title == "lambda parameter list expected" {
			lam = true
		}
		if strings.Contains(d.Message, "expected 'for' in list comprehension") {
			list = true
		}
		// NOTE: dict literal {k: v} is now valid syntax, no longer an error
		if strings.Contains(d.Message, "expected 'for' in set comprehension") {
			set = true
		}
	}
	if !lam || !list || !set {
		t.Fatalf("missing expected diagnostics: lambda:%v list:%v set:%v\nGot: %+v", lam, list, set, diags)
	}
}
