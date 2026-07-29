package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

// checkSrcDiags parses and checks src, failing the test on parse errors so that
// a check-phase assertion is never satisfied by a syntax error instead.
func checkSrcDiags(t *testing.T, name, src string) []diag.Diagnostic {
	t.Helper()
	mod, pdiags := parse.ParseFile(name, []byte(src))
	if len(pdiags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", pdiags)
	}
	diags, _ := Check(mod)
	return diags
}

// hasUndefinedName reports whether any diagnostic is DTE0001 naming want.
func hasUndefinedName(diags []diag.Diagnostic, want string) bool {
	for _, d := range diags {
		if d.CodeID == "DTE0001" && strings.Contains(d.Message, "undefined name '"+want+"'") {
			return true
		}
	}
	return false
}

// A name that resolves to nothing must be reported by the checker rather than
// surviving to the backend, which would emit a reference to a value it never
// defined and surface as a raw LLVM "use of undefined value" dump.
func TestUndefinedName_Reported(t *testing.T) {
	src := `
def main() -> int:
    let x = 1
    print(str(y))
    return 0
`
	diags := checkSrcDiags(t, "undefined_name.desi", src)
	if !hasUndefinedName(diags, "y") {
		t.Fatalf("expected DTE0001 for 'y', got: %+v", diags)
	}
}

// The names below are legitimately in scope but hold no symbol. Each used to
// reach typIdent and report, which is why the diagnostic could not simply be
// switched on. They are kept away from that path at the point of use, so none
// of these programs may produce an undefined-name diagnostic.
func TestUndefinedName_NotReportedForNonValuePositions(t *testing.T) {
	cases := []struct {
		name string
		miss string // the name that must not be reported
		src  string
	}{
		{
			// Built-in enum in qualifier position. Option has no symbol and
			// deliberately never gets one: it is resolved contextually from the
			// annotation at each use site.
			name: "option qualifier in call position",
			miss: "Option",
			src: `
def main() -> int:
    let x: Option<int> = Option.Some(42)
    return 0
`,
		},
		{
			name: "result qualifier in call position",
			miss: "Result",
			src: `
def main() -> int:
    let x: Result<int, str> = Result.Ok(100)
    return 0
`,
		},
		{
			// Field position rather than call position: a unit variant.
			name: "option qualifier in field position",
			miss: "Option",
			src: `
def main() -> int:
    let x: Option<int> = Option.Nothing
    return 0
`,
		},
		{
			// Arm patterns are pattern syntax, not expressions.
			name: "enum qualifier in match arm pattern",
			miss: "Option",
			src: `
def main() -> int:
    let x: Option<int> = Option.Some(42)
    match x:
        Option.Some(v): print("some")
        Option.Nothing: print("none")
    return 0
`,
		},
		{
			// The wildcard arm is not a name at all.
			name: "match wildcard",
			miss: "_",
			src: `
def main() -> int:
    let n = 3
    let s = match n:
        1: "one"
        _: "other"
    return 0
`,
		},
		{
			// An unqualified unit variant in an `is` pattern.
			name: "is pattern with unqualified unit variant",
			miss: "Nothing",
			src: `
def main() -> int:
    let x: Option<int> = Option.Nothing
    if x is Nothing:
        print("none")
    return 0
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := checkSrcDiags(t, "in_scope.desi", tc.src)
			if hasUndefinedName(diags, tc.miss) {
				t.Fatalf("%q is legitimately in scope but was reported undefined; diags: %+v", tc.miss, diags)
			}
		})
	}
}

// `pass` is a keyword. It was listed in token.keywords but missing from
// keywordToken in lex/scanner.go, so it lexed as an identifier: ast.PassStmt
// was unreachable and every `pass` in a function body reported as an undefined
// name once the diagnostic was switched on.
func TestUndefinedName_PassIsAStatement(t *testing.T) {
	src := `
def empty():
    pass

def main() -> int:
    empty()
    return 0
`
	diags := checkSrcDiags(t, "pass.desi", src)
	if hasUndefinedName(diags, "pass") {
		t.Fatalf("'pass' reported as an undefined name; diags: %+v", diags)
	}
}
