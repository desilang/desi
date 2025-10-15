package parse

import (
  "strings"
  "testing"

  "github.com/desilang/desi/compiler/internal/ast"
)

func renderClass(n ast.Node) string {
  var b strings.Builder
  ast.Print(&b, n)
  return strings.TrimSpace(b.String())
}

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
  // Nested class default visibility check
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

func TestParseClasses_M3A(t *testing.T) {
  // Tabs-only indentation on purpose.
  src := strings.Join([]string{
    "@trace",
    "class Animal:",
    "\t\"\"\"base class doc\"\"\"",
    "\tpub species: str",
    "",
    "\t@logged",
    "\tpub def speak() -> str: \"???\"",
    "",
    "class Dog(Animal):",
    "\tpub breed: str",
    "\tdef speak() -> str: \"woof\"",
    "",
    "\t@bench",
    "\tclass Meta:",
    "\t\tpub version: str",
    "",
  }, "\n")

  mod, diags := ParseFile("<mem>", []byte(src))
  if len(diags) != 0 {
    t.Fatalf("unexpected diagnostics: %+v", diags)
  }

  got := renderClass(mod)

  // Printer order is: DocString -> Fields -> Nested Classes -> Methods
  // It also prints 'pub' on methods.
  want := strings.TrimSpace(`
Module("<mem>")
	@trace
	Class pub Animal
		DocString
		Field pub species: str
		@logged
		Func pub speak() -> str
			Block
				Str
	Class pub Dog(Animal)
		Field pub breed: str
		@bench
		Class Meta
			Field pub version: str
		Func speak() -> str
			Block
				Str
`)

  if got != want {
    t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
  }
}
