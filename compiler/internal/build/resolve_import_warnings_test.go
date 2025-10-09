// compiler/internal/build/resolve_import_warnings_test.go
package build

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/loaderutil"
)

func codes(diags []diag.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func has(diags []diag.Diagnostic, code string, domain string) bool {
	for _, d := range diags {
		if d.Code == code && d.Domain == domain {
			return true
		}
	}
	return false
}

func TestUnusedPlainImport_NoAlias(t *testing.T) {
	src := `
import util.math

def main() -> int:
  return 0
`
	imps := []loaderutil.ImportDecl{
		{Kind: loaderutil.ImportPlain, Module: "util.math"},
	}

	diags := loaderutil.ScanUnusedAndDuplicateImports(src, imps)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag, got %d: %#v", len(diags), codes(diags))
	}
	if !has(diags, "DMW0004", "module") {
		t.Fatalf("expected DMW0004 unused import, got: %#v", codes(diags))
	}
}

func TestUnusedPlainImport_WithAlias(t *testing.T) {
	src := `
import util.math as m

def main() -> int:
  return 0
`
	imps := []loaderutil.ImportDecl{
		{Kind: loaderutil.ImportPlain, Module: "util.math", Alias: "m"},
	}

	diags := loaderutil.ScanUnusedAndDuplicateImports(src, imps)
	if len(diags) != 1 || !has(diags, "DMW0004", "module") {
		t.Fatalf("expected DMW0004 unused import for alias, got: %#v", codes(diags))
	}
}

func TestFromImport_AllUnused_AsSingleUnusedImport(t *testing.T) {
	src := `
from util.math import add, sub

def main() -> int:
  return 0
`
	imps := []loaderutil.ImportDecl{
		{
			Kind:   loaderutil.ImportFrom,
			Module: "util.math",
			Items: []loaderutil.FromItem{
				{Name: "add"},
				{Name: "sub"},
			},
		},
	}

	diags := loaderutil.ScanUnusedAndDuplicateImports(src, imps)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag, got %d: %#v", len(diags), codes(diags))
	}
	if !has(diags, "DMW0004", "module") {
		t.Fatalf("expected DMW0004 (all unused), got: %#v", codes(diags))
	}
}

func TestFromImport_SomeUnused_Itemized(t *testing.T) {
	src := `
from util.math import add, sub

def main() -> int:
  return add(2, 3)
`
	imps := []loaderutil.ImportDecl{
		{
			Kind:   loaderutil.ImportFrom,
			Module: "util.math",
			Items: []loaderutil.FromItem{
				{Name: "add"},
				{Name: "sub"},
			},
		},
	}

	diags := loaderutil.ScanUnusedAndDuplicateImports(src, imps)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag (one unused item), got %d: %#v", len(diags), codes(diags))
	}
	if !has(diags, "DMW0005", "module") {
		t.Fatalf("expected DMW0005 for the unused item, got: %#v", codes(diags))
	}
}

func TestFromImport_DuplicateItems(t *testing.T) {
	src := `
from util.math import sqrt, sqrt

def main() -> int:
  return sqrt(9)
`
	imps := []loaderutil.ImportDecl{
		{
			Kind:   loaderutil.ImportFrom,
			Module: "util.math",
			Items: []loaderutil.FromItem{
				{Name: "sqrt"},
				{Name: "sqrt"},
			},
		},
	}

	diags := loaderutil.ScanUnusedAndDuplicateImports(src, imps)
	// Expect one duplicate warning (on the second occurrence)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag (duplicate item), got %d: %#v", len(diags), codes(diags))
	}
	if !has(diags, "DMW0006", "module") {
		t.Fatalf("expected DMW0006 duplicate-from-item, got: %#v", codes(diags))
	}
}

func TestNoFalsePositives_PlainAliasAndFromUsed(t *testing.T) {
	src := `
import util.math as m
from util.math import add, sub

def main() -> int:
  x = m.add(1, 2)
  y = sub(5, 3)
  return add(x, y)
`
	imps := []loaderutil.ImportDecl{
		{Kind: loaderutil.ImportPlain, Module: "util.math", Alias: "m"},
		{
			Kind:   loaderutil.ImportFrom,
			Module: "util.math",
			Items: []loaderutil.FromItem{
				{Name: "add"},
				{Name: "sub"},
			},
		},
	}

	diags := loaderutil.ScanUnusedAndDuplicateImports(src, imps)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diags, got %d: %#v", len(diags), codes(diags))
	}
}
