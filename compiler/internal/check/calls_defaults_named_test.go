package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
)

// Local function: positional calls using a defaulted parameter.
func TestM14_LocalDefaults_Positional_OK(t *testing.T) {
	src := `
def f(a: int, b: int = 1) -> int:
  a + b

def main() -> int:
  f(1)
  f(1, 2)
  0
`
	mod, diags := parse.ParseFile("defaults_local_pos.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	diags2, _ := Check(mod)
	if len(diags2) != 0 {
		t.Fatalf("unexpected check diags: %+v", diags2)
	}
}

// Local function: named-arg calls, including reordering + using default for b.
func TestM14_LocalDefaults_Named_OK(t *testing.T) {
	src := `
def f(a: int, b: int = 1) -> int:
  a + b

def main() -> int:
  f(a=1)
  f(b=2, a=1)
  0
`
	mod, diags := parse.ParseFile("defaults_local_named.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	diags2, _ := Check(mod)
	if len(diags2) != 0 {
		t.Fatalf("unexpected check diags: %+v", diags2)
	}
}

// Local function: missing required param even with defaults should fail.
func TestM14_LocalDefaults_Named_MissingRequired(t *testing.T) {
	src := `
def f(a: int, b: int = 1) -> int:
  a + b

def main() -> int:
  f(b=2)
  0
`
	mod, diags := parse.ParseFile("defaults_local_missing.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	diags2, _ := Check(mod)
	if len(diags2) == 0 {
		t.Fatalf("expected diagnostics, got none")
	}
	// canonicalizeForCandidate will reject this, and the fallback path should
	// report a no-matching-overload-style error.
	mustHaveSomeDiagContaining(t, diags2, "no matching overload")
}

// Cross-module: defaults + named args on an imported function.
func TestM14_FromImport_Defaults_Named_OK(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def add(a: int, b: int = 1) -> int:
  a + b
`,
		"main.desi": `
from math import add

def main() -> int:
  add(a=1)
  add(b=2, a=1)
  0
`,
	})

	mod, diags := parse.ParseFile("main.desi", []byte(ldr.Files["main.desi"]))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	res := CheckWithLoader(mod, ldr)
	if len(res.Diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", res.Diags)
	}
}
