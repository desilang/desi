package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseValid(t *testing.T) {
	dir := t.TempDir()

	// Create the manifest-declared roots and entry so Load() validation passes.
	writeTemp(t, dir, "src/main.desi", "def main() -> int:\n  0\n")

	mod := `
[package]
name    = "hello-desi"
version = "0.1.0"
edition = "2025"
entry   = "src/main.desi"
roots   = ["src"]

[build]
mode    = "debug"
out_dir = "build"

[target]
triple  = "native"

[diagnostics]
error_format = "human"
color        = "auto"
max_errors   = "100"

[ffi]
libs   = ["m"]
search = []

[[ffi.extern]]
name = "sqrt"
lib  = "m"
`
	mp := writeTemp(t, dir, "desi.mod", mod)

	m, diags := Load(mp)
	if len(diags) > 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if m.Package.Name != "hello-desi" || m.EntryPath() == "" || len(m.Roots()) != 1 {
		t.Fatalf("bad manifest fields: %+v", m)
	}
	dd := m.DiagDefaults()
	if dd.ErrorFormat != "human" || dd.Color != "auto" || dd.MaxErrors != 100 {
		t.Fatalf("bad diag defaults: %+v", dd)
	}
}

func TestUnknownSectionAndKey(t *testing.T) {
	dir := t.TempDir()
	// No roots/entry created here on purpose; we only assert "some diags".
	mod := `
[unknown]
foo = "bar"

[package]
name = "x"
entry = "main.desi"
roots = ["src"]
`
	mp := writeTemp(t, dir, "desi.mod", mod)
	_, diags := Load(mp)
	if len(diags) == 0 {
		t.Fatalf("expected diags for unknown section/key")
	}
}

func TestFindRoot(t *testing.T) {
	top := t.TempDir()
	inner := filepath.Join(top, "a", "b")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, top, "desi.mod", `[package]
name="x"
entry="src/main.desi"
roots=["src"]`)
	root, path, ok := FindRoot(inner)
	if !ok || root != top || path != filepath.Join(top, "desi.mod") {
		t.Fatalf("FindRoot failed: %v %v %v", root, path, ok)
	}
}
