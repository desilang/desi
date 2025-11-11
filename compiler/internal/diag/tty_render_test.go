package diag_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	d "github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

// in-memory source provider for deterministic snippets
type memSource map[string][]byte

func (m memSource) File(path string) ([]byte, bool) {
	b, ok := m[path]
	return b, ok
}

func normalizeNL(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func TestTTYSnapshot_UnexpectedToken(t *testing.T) {
	inPath := filepath.Join("testdata", "tty", "unexpected.in.desi")
	outPath := filepath.Join("testdata", "tty", "unexpected.out.txt")

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
	ms := memSource{"<stdin>": src}
	opt := d.Options{Color: d.Never, Width: 80, ExpandTabs: 8}

	if err := d.RenderTTYWith(&buf, diags[0], ms, opt); err != nil {
		t.Fatalf("RenderTTYWith: %v", err)
	}
	got := normalizeNL(buf.String())

	if got != want {
		t.Fatalf("TTY snapshot mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
