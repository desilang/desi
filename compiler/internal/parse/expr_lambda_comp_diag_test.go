package parse

import (
	"strings"
	"testing"
)

func TestDiagnostics_LambdaHeadAndCompHead(t *testing.T) {
	src := `
let badLam = (f(x)) => x   # invalid lambda head before =>
let a = [x]                # missing 'for'
let b = {k: v}             # missing 'for'
let c = #{x}               # missing 'for'
`
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) < 4 {
		t.Fatalf("expected at least 4 diagnostics, got %d", len(diags))
	}
	// We don't assert strict ordering; just that the expected messages appear.
	var lam, list, dict, set bool
	for _, d := range diags {
		if d.Title == "lambda parameter list expected" {
			lam = true
		}
		if strings.Contains(d.Message, "expected 'for' in list comprehension") {
			list = true
		}
		if strings.Contains(d.Message, "expected 'for' in dict comprehension") {
			dict = true
		}
		if strings.Contains(d.Message, "expected 'for' in set comprehension") {
			set = true
		}
	}
	if !lam || !list || !dict || !set {
		t.Fatalf("missing expected diagnostics: lambda:%v list:%v dict:%v set:%v\nGot: %+v", lam, list, dict, set, diags)
	}
}
