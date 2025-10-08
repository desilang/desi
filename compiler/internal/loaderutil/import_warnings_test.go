package loaderutil_test

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/loaderutil"
)

func collectCodes(es []error) []string {
	var out []string
	for _, e := range es {
		switch d := e.(type) {
		case diag.Diagnostic:
			out = append(out, d.Code)
		case *diag.Diagnostic:
			out = append(out, d.Code)
		}
	}
	return out
}

func hasCode(es []error, want string) bool {
	for _, c := range collectCodes(es) {
		if c == want {
			return true
		}
	}
	return false
}

func TestScanSelfImport_ImportAndFrom(t *testing.T) {
	module := "app.main"
	f := &ast.File{
		Imports: []ast.ImportDecl{
			{Path: module}, // import app.main  (self-import)
		},
		FromImports: []ast.FromImportDecl{
			{Module: module, Items: []ast.ImportItem{{Name: "x"}}}, // from app.main import x (self-from-import)
		},
	}

	var diags []error
	diags = append(diags, loaderutil.ScanSelfImport(f, module)...)

	if !hasCode(diags, "DMW0002") {
		t.Fatalf("expected DMW0002 (module imports itself), got codes: %v", collectCodes(diags))
	}
	// We emitted for both the direct import and the from-import; ensure we have at least 2 diags total.
	if got := len(diags); got < 2 {
		t.Fatalf("expected >= 2 self-import diagnostics, got %d", got)
	}
}

func TestScanImportAliasConflicts_Simple(t *testing.T) {
	// conflict on local alias "io"
	f := &ast.File{
		Imports: []ast.ImportDecl{
			{Path: "foo.io", As: "io"}, // introduces local "io"
		},
		FromImports: []ast.FromImportDecl{
			{
				Module: "bar.zip",
				Items:  []ast.ImportItem{{Name: "io"}}, // introduces local "io" again (default to Name)
			},
		},
	}

	diags := loaderutil.ScanImportAliasConflicts(f)
	if !hasCode(diags, "DMW0003") {
		t.Fatalf("expected DMW0003 (import alias conflict), got codes: %v", collectCodes(diags))
	}
}

func TestScanImportAliasConflicts_None(t *testing.T) {
	// no conflict: different local names
	f := &ast.File{
		Imports: []ast.ImportDecl{
			{Path: "foo.io", As: "io1"},
		},
		FromImports: []ast.FromImportDecl{
			{
				Module: "bar.zip",
				Items:  []ast.ImportItem{{Name: "io", As: "io2"}},
			},
		},
	}

	diags := loaderutil.ScanImportAliasConflicts(f)
	if len(diags) != 0 {
		t.Fatalf("expected 0 alias-conflict diagnostics, got %d, codes: %v", len(diags), collectCodes(diags))
	}
}
