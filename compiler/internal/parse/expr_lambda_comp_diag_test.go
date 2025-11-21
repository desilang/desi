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
let c = #{x}               # VALID set literal now (not an error!)
`
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) < 2 {
		t.Fatalf("expected at least 2 diagnostics, got %d", len(diags))
	}
	// We don't assert strict ordering; just that the expected messages appear.
	var lam, list bool
	for _, d := range diags {
		if d.Title == "lambda parameter list expected" {
			lam = true
		}
		if strings.Contains(d.Message, "expected 'for' in list comprehension") {
			list = true
		}
		// NOTE: #{x} is now a valid set literal, so we don't check for set comp error
	}
	if !lam || !list {
		t.Fatalf("missing expected diagnostics: lambda:%v list:%v\nGot: %+v", lam, list, diags)
	}
}
