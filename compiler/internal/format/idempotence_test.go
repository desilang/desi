package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: load *.in.desi files from a dir and run idempotence check
func runIdempotenceOnDir(t *testing.T, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".in.desi") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		out1, diags1 := FormatBytes(src)
		if len(diags1) > 0 {
			t.Fatalf("%s: parse diags on first format: %v", name, diags1)
		}
		out2, diags2 := FormatBytes(out1)
		if len(diags2) > 0 {
			t.Fatalf("%s: parse diags on second format: %v", name, diags2)
		}
		if string(out1) != string(out2) {
			t.Fatalf("%s: format is not idempotent", name)
		}
		// trailing newline required
		if len(out1) == 0 || out1[len(out1)-1] != '\n' {
			t.Fatalf("%s: formatted output must end with a single newline", name)
		}
	}
}

func TestIdempotence_Phase1(t *testing.T) {
	dir := filepath.Join("testdata", "phase1")
	runIdempotenceOnDir(t, dir)
}

func TestIdempotence_Phase2(t *testing.T) {
	dir := filepath.Join("testdata", "phase2")
	runIdempotenceOnDir(t, dir)
}
