package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parse"
)

// TestDisplayTrait_CustomImpl tests that custom impl Display works
func TestDisplayTrait_CustomImpl(t *testing.T) {
	src := `
trait Display:
	def to_str() -> str

struct Point:
	x: int
	y: int

impl Display for Point:
	def to_str() -> str:
		return "Point"

def main():
	let p = Point(x=10, y=20)
	let s = p.to_str()
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) > 0 {
		t.Fatalf("parse errors: %v", pdiags)
	}

	diags, info := Check(mod)
	if len(diags) > 0 {
		t.Fatalf("check errors: %v", diags)
	}

	// Verify Point has Display impl
	if info.Impls == nil {
		t.Fatal("Info.Impls is nil")
	}
	if _, hasPoint := info.Impls["Point"]; !hasPoint {
		t.Fatal("Point not in Impls")
	}
	if _, hasDisplay := info.Impls["Point"]["Display"]; !hasDisplay {
		t.Fatal("Point does not have Display impl")
	}

	methods := info.Impls["Point"]["Display"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if methods[0].Name.Name != "to_str" {
		t.Errorf("expected method name 'to_str', got '%s'", methods[0].Name.Name)
	}
}

// TestDisplayTrait_DefaultImpl tests that default impl Display is auto-generated
func TestDisplayTrait_DefaultImpl(t *testing.T) {
	src := `
struct Point:
	x: int
	y: int

def main():
	let p = Point(x=10, y=20)
	let s = p.to_str()
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) > 0 {
		t.Fatalf("parse errors: %v", pdiags)
	}

	diags, info := Check(mod)
	if len(diags) > 0 {
		t.Fatalf("check errors: %v", diags)
	}

	// Verify Point has auto-generated Display impl
	if info.Impls == nil {
		t.Fatal("Info.Impls is nil")
	}
	if _, hasPoint := info.Impls["Point"]; !hasPoint {
		t.Fatal("Point not in Impls (should have auto-generated Display)")
	}
	if _, hasDisplay := info.Impls["Point"]["Display"]; !hasDisplay {
		t.Fatal("Point does not have Display impl (should be auto-generated)")
	}

	methods := info.Impls["Point"]["Display"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 auto-generated method, got %d", len(methods))
	}
	if methods[0].Name.Name != "to_str" {
		t.Errorf("expected auto-generated method name 'to_str', got '%s'", methods[0].Name.Name)
	}

	// Auto-generated method should have nil body
	if methods[0].Body != nil {
		t.Error("auto-generated to_str should have nil body")
	}
}

// TestDisplayTrait_MethodCall tests that obj.to_str() resolves correctly
func TestDisplayTrait_MethodCall(t *testing.T) {
	src := `
struct Point:
	x: int
	y: int

impl Display for Point:
	def to_str() -> str:
		return "Point"

def main():
	let p = Point(x=10, y=20)
	let s = p.to_str()
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) > 0 {
		t.Fatalf("parse errors: %v", pdiags)
	}

	diags, info := Check(mod)
	if len(diags) > 0 {
		t.Fatalf("check errors: %v", diags)
	}

	// Find the call to p.to_str() and check its type
	var foundCall *ast.CallExpr
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "main" && fn.Body != nil {
			for _, stmt := range fn.Body.Stmts {
				if ls, ok := stmt.(*ast.LetStmt); ok && ls.Name.Name == "s" {
					if call, ok := ls.Value.(*ast.CallExpr); ok {
						foundCall = call
						break
					}
				}
			}
		}
	}

	if foundCall == nil {
		t.Fatal("could not find p.to_str() call")
	}

	// Check that the call has type str
	callType := info.Types[foundCall]
	if callType == nil {
		t.Fatal("call to p.to_str() has no type")
	}
	if callType.String() != "str" {
		t.Errorf("expected call type 'str', got '%s'", callType.String())
	}
}

// TestDisplayTrait_OverrideDefault tests that explicit impl overrides auto-generated
func TestDisplayTrait_OverrideDefault(t *testing.T) {
	src := `
struct Point:
	x: int
	y: int

impl Display for Point:
	def to_str() -> str:
		return "CustomPoint"

def main():
	let p = Point(x=10, y=20)
	let s = p.to_str()
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) > 0 {
		t.Fatalf("parse errors: %v", pdiags)
	}

	diags, info := Check(mod)
	if len(diags) > 0 {
		t.Fatalf("check errors: %v", diags)
	}

	methods := info.Impls["Point"]["Display"]
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}

	// Explicit impl should have a body (not auto-generated)
	if methods[0].Body == nil {
		t.Error("explicit impl to_str should have a body")
	}
}

// TestDisplayTrait_ParseError tests that invalid trait syntax is caught
func TestDisplayTrait_ParseError(t *testing.T) {
	// Test that trait methods with bodies are rejected
	src := `
trait Display:
	def to_str() -> str:
		return "invalid"
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) > 0 {
		// Parser might catch this
		return
	}

	diags, _ := Check(mod)
	// Should get checker error about trait methods having bodies
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "trait methods") || strings.Contains(d.Message, "bodies") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about trait methods with bodies")
	}
}

// TestDisplayTrait_PrintIntegration tests that print(Display) works
func TestDisplayTrait_PrintIntegration(t *testing.T) {
	src := `
struct Point:
	x: int
	y: int

def main():
	let p = Point(x=10, y=20)
	print(p)
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) > 0 {
		t.Fatalf("parse errors: %v", pdiags)
	}

	diags, info := Check(mod)
	if len(diags) > 0 {
		t.Fatalf("check errors: %v", diags)
	}

	// Verify Point has auto-generated Display
	if _, hasDisplay := info.Impls["Point"]["Display"]; !hasDisplay {
		t.Fatal("Point should have auto-generated Display")
	}

	// Find the print(p) call and verify it type-checks
	var foundPrintCall *ast.CallExpr
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "main" && fn.Body != nil {
			for _, stmt := range fn.Body.Stmts {
				if es, ok := stmt.(*ast.ExprStmt); ok {
					if call, ok := es.Expr.(*ast.CallExpr); ok {
						if id, ok := call.Callee.(*ast.Ident); ok && id.Name == "print" {
							foundPrintCall = call
							break
						}
					}
				}
			}
		}
	}

	if foundPrintCall == nil {
		t.Fatal("could not find print(p) call")
	}

	// Verify the call has type none
	callType := info.Types[foundPrintCall]
	if callType == nil {
		t.Fatal("print(p) call has no type")
	}
	if callType.String() != "none" {
		t.Errorf("expected call type 'none', got '%s'", callType.String())
	}
}
