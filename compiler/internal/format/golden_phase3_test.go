package format

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGolden_Phase3(t *testing.T) {
	td := filepath.Join("testdata", "phase3")
	entries, err := os.ReadDir(td)
	if err != nil {
		t.Fatalf("read %s: %v", td, err)
	}
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".desi" || !bytes.HasSuffix([]byte(name), []byte(".in.desi")) {
			continue
		}
		inPath := filepath.Join(td, name)
		goldPath := filepath.Join(td, replaceSuffix(name, ".in.desi", ".golden.desi"))

		inb, err := os.ReadFile(inPath)
		if err != nil {
			t.Fatalf("read %s: %v", inPath, err)
		}
		want, err := os.ReadFile(goldPath)
		if err != nil {
			t.Fatalf("read %s: %v", goldPath, err)
		}
		got, diags := FormatBytes(inb)
		if len(diags) != 0 {
			t.Fatalf("%s: parse diags: %+v", name, diags)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s: golden mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
		}
	}
}

func TestIdempotence_Phase3(t *testing.T) {
	td := filepath.Join("testdata", "phase3")
	entries, err := os.ReadDir(td)
	if err != nil {
		t.Fatalf("read %s: %v", td, err)
	}
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".desi" || !bytes.HasSuffix([]byte(name), []byte(".in.desi")) {
			continue
		}
		inPath := filepath.Join(td, name)
		inb, err := os.ReadFile(inPath)
		if err != nil {
			t.Fatalf("read %s: %v", inPath, err)
		}
		out1, diags1 := FormatBytes(inb)
		if len(diags1) != 0 {
			t.Fatalf("%s: parse diags pass1: %+v", name, diags1)
		}
		out2, diags2 := FormatBytes(out1)
		if len(diags2) != 0 {
			t.Fatalf("%s: parse diags pass2: %+v", name, diags2)
		}
		if !bytes.Equal(out1, out2) {
			t.Fatalf("%s: format is not idempotent", name)
		}
	}
}

func replaceSuffix(s, old, new string) string {
	if len(s) >= len(old) && s[len(s)-len(old):] == old {
		return s[:len(s)-len(old)] + new
	}
	return s
}
