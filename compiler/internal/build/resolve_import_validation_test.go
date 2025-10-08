package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
)

func hasDiag(diags []diag.Diagnostic, key string) bool {
	for _, d := range diags {
		if d.Domain == "module" && d.Key == key {
			return true
		}
	}
	return false
}

func TestResolve_BadImport_IsFlagged(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.desi")
	src := "import 123bad\n" +
		"def main() -> int:\n" +
		"  return 0\n"
	if err := os.WriteFile(entry, []byte(src), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	_, diags, err := ResolveEntry(entry, ResolveOptions{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !hasDiag(diags, "bad_import") {
		t.Fatalf("expected module.bad_import diagnostic, got: %#v", diags)
	}
}

func TestResolve_NotFound_ValidModule(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.desi")
	src := "import foo.bar\n" +
		"def main() -> int:\n" +
		"  return 0\n"
	if err := os.WriteFile(entry, []byte(src), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	_, diags, err := ResolveEntry(entry, ResolveOptions{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !hasDiag(diags, "not_found") {
		t.Fatalf("expected module.not_found diagnostic, got: %#v", diags)
	}
}
