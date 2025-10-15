package parse

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM3A_ClassParse_CoreBits(t *testing.T) {
	src := `
@trace
pub class Animal:
	"""base class doc"""
	pub species: str

	@logged
	pub def speak() -> str: "???"

class Dog(Animal):
	pub breed: str
	def speak() -> str: "woof"

	@bench
	class Meta:
		pub version: str
`
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	// Expect two top-level class decls
	if len(mod.Decls) != 2 {
		t.Fatalf("expected 2 top-level decls, got %d", len(mod.Decls))
	}
	animal, _ := mod.Decls[0].(*ast.ClassDecl)
	dog, _ := mod.Decls[1].(*ast.ClassDecl)
	if animal == nil || dog == nil {
		t.Fatalf("expected ClassDecls at top-level")
	}

	// Animal: pub via explicit keyword; has decorator and docstring
	if !animal.Pub {
		t.Fatalf("Animal should be public")
	}
	if len(animal.Decorators) != 1 || animal.Decorators[0].Name.Name != "trace" {
		t.Fatalf("missing/invalid class decorator on Animal")
	}
	if animal.Doc == nil || !animal.Doc.Long {
		t.Fatalf("Animal docstring not attached")
	}
	if len(animal.Fields) != 1 || !animal.Fields[0].Pub || animal.Fields[0].Name.Name != "species" {
		t.Fatalf("Animal field not parsed with pub/type")
	}
	if len(animal.Methods) != 1 {
		t.Fatalf("expected one method on Animal")
	}
	m := animal.Methods[0]
	if !m.Pub || m.Name.Name != "speak" {
		t.Fatalf("Animal.speak should be pub method")
	}
	if m.RetType == nil || m.RetType.Name != "str" {
		t.Fatalf("Animal.speak should have return type str")
	}
	if len(m.Decorators) != 1 || m.Decorators[0].Name.Name != "logged" {
		t.Fatalf("Animal.speak decorator missing")
	}

	// Dog: public by default (top-level) even without 'pub'
	if !dog.Pub {
		t.Fatalf("Dog should be public by default (top-level)")
	}
	if len(dog.Bases) != 1 || dog.Bases[0].Name != "Animal" {
		t.Fatalf("Dog base list incorrect: %+v", dog.Bases)
	}
	if len(dog.Fields) != 1 || !dog.Fields[0].Pub || dog.Fields[0].Name.Name != "breed" {
		t.Fatalf("Dog field 'breed' not parsed with pub")
	}
	// Dog.speak is private by default (no 'pub')
	foundMeta := false
	for _, c := range dog.Nested {
		if c.Name.Name == "Meta" {
			foundMeta = true
			if c.Pub {
				t.Fatalf("Nested class Meta should be private by default")
			}
			if len(c.Fields) != 1 || !c.Fields[0].Pub || c.Fields[0].Name.Name != "version" {
				t.Fatalf("Meta.field 'version' not parsed with pub")
			}
		}
	}
	if !foundMeta {
		t.Fatalf("nested class Meta not found")
	}
}
