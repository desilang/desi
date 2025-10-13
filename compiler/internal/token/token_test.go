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
	if GT.String() != ">" {
		t.Fatalf("GT string wrong: %q", GT.String())
	}
}
