package parse

import "testing"

func TestUnsafeBlock_Parse(t *testing.T) {
	src := "def f():\n\tunsafe:\n\t\tlet x = 1\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestUnsafeBlock_MissingColon_Diag(t *testing.T) {
	src := "def f():\n\tunsafe\n\t\tlet x = 1\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) == 0 {
		t.Fatalf("expected a diagnostic for missing ':' after unsafe")
	}
}
