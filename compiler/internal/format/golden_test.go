package format

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGolden_Phase1(t *testing.T) { runGoldenDir(t, "testdata/phase1") }
func TestGolden_Phase2(t *testing.T) { runGoldenDir(t, "testdata/phase2") }

func runGoldenDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".in.desi") {
			continue
		}
		base := strings.TrimSuffix(name, ".in.desi")
		inPath := filepath.Join(dir, base+".in.desi")
		goldenPath := filepath.Join(dir, base+".golden.desi")

		in, err := os.ReadFile(inPath)
		if err != nil {
			t.Fatalf("read %s: %v", inPath, err)
		}
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read %s: %v", goldenPath, err)
		}

		got, diags := FormatBytes(in)
		if len(diags) != 0 {
			t.Fatalf("%s: parse diags: %+v", base, diags)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("%s: mismatch.\n--- got ---\n%s\n--- want ---\n%s", base, got, want)
		}

		// idempotence
		got2, diags2 := FormatBytes(got)
		if len(diags2) != 0 {
			t.Fatalf("%s: parse diags (second pass): %+v", base, diags2)
		}
		if !bytes.Equal(got, got2) {
			t.Fatalf("%s: not idempotent", base)
		}
	}
}
