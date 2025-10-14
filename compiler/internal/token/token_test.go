package token

import "testing"

func TestHelpers(t *testing.T) {
	if !IsKeyword("def") {
		t.Fatalf("expected def to be a keyword")
	}
	if !IsBuiltinType("int") {
		t.Fatalf("expected int to be a builtin type")
	}
	if IsBuiltinType("wat") {
		t.Fatalf("wat should not be a builtin type")
	}
	if !IsOperator(PLUS) || IsOperator(LPAREN) {
		t.Fatalf("operator classification failed")
	}
	if TokenCategory(KW_def) != CatKeyword {
		t.Fatalf("KW_def should be CatKeyword")
	}

	// String() is the symbolic/stable name:
	if GT.String() != "GT" {
		t.Fatalf("GT String wrong: %q", GT.String())
	}
	// Lit() is the canonical source spelling (if any):
	if GT.Lit() != ">" {
		t.Fatalf("GT Lit wrong: %q", GT.Lit())
	}
	if ARROW.Lit() != "->" {
		t.Fatalf("ARROW Lit wrong: %q", ARROW.Lit())
	}
	if KW_def.Lit() != "def" {
		t.Fatalf("keyword Lit wrong: %q", KW_def.Lit())
	}
	if INT_DEC.Lit() != "" {
		t.Fatalf("INT_DEC should not have a fixed literal")
	}
}
