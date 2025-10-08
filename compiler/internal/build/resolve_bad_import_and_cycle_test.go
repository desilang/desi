package build

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLocal(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestBadFromImportVariants(t *testing.T) {
	cases := []struct {
		name string
		head string // the 'from ... import ...' line
	}{
		{"starts_with_digit", "from 123bad import thing"},
		{"contains_hyphen", "from util.math-extra import sqrt"},
		{"double_dot_empty_seg", "from std..io import write_line"},
		{"trailing_dot", "from foo. import bar"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			entry := writeLocal(t, dir, "main.desi", tc.head+"\n\ndef main() -> int:\n  return 0\n")

			_, diags, err := ResolveEntry(entry, ResolveOptions{})
			if err != nil {
				t.Fatalf("ResolveEntry fatal: %v", err)
			}
			found := false
			for _, d := range diags {
				if d.Domain == "module" && d.Key == "bad_import" && d.Code != "" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected module.bad_import for case %q; diags=%v", tc.name, diags)
			}
		})
	}
}

func TestMiniImportCycle(t *testing.T) {
	dir := t.TempDir()
	writeLocal(t, dir, "a.desi", "import b\n\ndef main() -> int:\n  return 0\n")
	writeLocal(t, dir, "b.desi", "import a\n")

	entry := filepath.Join(dir, "a.desi")
	plan, diags, err := ResolveEntry(entry, ResolveOptions{})
	if err != nil {
		t.Fatalf("ResolveEntry fatal: %v", err)
	}
	// Expect an import cycle diagnostic
	gotCycle := false
	for _, d := range diags {
		if d.Domain == "module" && d.Key == "import_cycle" {
			gotCycle = true
			break
		}
	}
	if !gotCycle {
		t.Fatalf("expected module.import_cycle; diags=%v", diags)
	}

	// With a cycle, we currently return a partial plan (entry only).
	if len(plan.Deps) == 0 || filepath.Clean(plan.Deps[0].File) != filepath.Clean(entry) {
		t.Fatalf("expected plan to include entry first; got: %+v", plan)
	}
}
