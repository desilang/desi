package parse

import (
	"testing"
)

func TestDiagnostics_LambdaHeadAndCompHead(t *testing.T) {
	src := `
let badLam = (f(x)) => x   # invalid lambda head before =>
let a = [x]                # VALID: single-element list literal now
let b = {k: v}             # VALID dict literal now (not an error!)
let c = #{x}               # VALID set literal now (not an error!)
`
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) < 1 {
		t.Fatalf("expected at least 1 diagnostic, got %d", len(diags))
	}
	// We only check for lambda error now; [x] is a valid list literal
	var lam bool
	for _, d := range diags {
		if d.Title == "lambda parameter list expected" {
			lam = true
		}
	}
	if !lam {
		t.Fatalf("missing expected lambda diagnostic\nGot: %+v", diags)
	}
}
