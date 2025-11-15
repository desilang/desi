package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
)

func TestM14_Defaults_Overloads_Consistent_OK(t *testing.T) {
	// Two overloads with *different* parameter types but the same default mask:
	//
	//   f(int,  int  = 1)
	//   f(float, float = 1.0)
	//
	// That should be:
	//   - allowed by the default-consistency rule (same default pattern),
	//   - non-ambiguous at call sites, because argument types differ.
	src := `
def f(a: int, b: int = 1) -> int:
  a + b

def f(a: float, b: float = 1.0) -> float:
  a + b

def main() -> int:
  f(1)
  f(1.0)
  0
`
	mod, diags := parse.ParseFile("over_ok.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	diags2, _ := Check(mod)
	if len(diags2) != 0 {
		t.Fatalf("unexpected check diags: %+v", diags2)
	}
}

func TestM14_Defaults_Overloads_Inconsistent(t *testing.T) {
	// Overloads disagree on which parameter has a default:
	//
	//   f(a: int, b: int = 1)
	//   f(a: int, b: int)
	//
	// That should trigger DDF0005 ("default parameters must be consistent
	// across overloads of f") in addition to any call-site noise.
	src := `
def f(a: int, b: int = 1) -> int:
  a + b

def f(a: int, b: int) -> int:
  a + b

def main() -> int:
  f(1)
  0
`
	mod, diags := parse.ParseFile("over_bad.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	diags2, _ := Check(mod)
	if len(diags2) == 0 {
		t.Fatalf("expected diagnostics, got none")
	}
	mustHaveSomeDiagContaining(t, diags2, "default parameters must be consistent across overloads")
}
