package parse

import "testing"

// Just make sure core M2 constructs parse without diagnostics.
// Detailed AST shape is exercised via -ast and the printer.

func TestM2_AugAssigns_Parse(t *testing.T) {
	src := "def f():\n\tlet a = 1\n\ta += 2\n\ta **= 3\n\ta, b := 10, 20\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestM2_InlineFlow_Parse(t *testing.T) {
	src := "def g():\n\tif cond: return 1\n\twhile cond: return 2\n\tfor i in xs: return i\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestM2_UsingDefer_Parse(t *testing.T) {
	src := "def h():\n\tusing handle = open():\n\t\tdefer handle.close()\n\t\treturn 0\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestM2_Docstring_LongOnly(t *testing.T) {
	// Only triple-quoted docstrings count; a plain "..." would be an ExprStmt.
	src := "def w():\n\t\"\"\"hi\"\"\"\n\treturn 1\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}
