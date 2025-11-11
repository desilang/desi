package diag_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	d "github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

func TestJSONSnapshot_UnexpectedToken(t *testing.T) {
	inPath := filepath.Join("testdata", "json", "unexpected.in.desi")
	outPath := filepath.Join("testdata", "json", "unexpected.out.json")

	src, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	wantBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	want := normalizeNL(string(wantBytes))

	_, diags := parse.ParseFile("<stdin>", src)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics, got none")
	}

	var buf bytes.Buffer
	if err := d.EncodeJSON(&buf, diags[:1]); err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	got := normalizeNL(buf.String())

	if got != want {
		t.Fatalf("JSON snapshot mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
