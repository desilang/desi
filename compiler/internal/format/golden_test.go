package format

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/term"
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
			term.Eprintf(">> %s: parse diags on input:\n%+v\n", base, diags)
			term.Flush()
			t.Fatalf("%s: parse diags: %+v", base, diags)
		}

		if !bytes.Equal(got, want) {
			term.Eprintln("========================================")
			term.Eprintf("%s: GOLDEN MISMATCH\n", base)
			term.Eprintln("---- INPUT (visible) ----")
			term.Eprint(visibleWithLineNos(in))
			term.Eprintln("---- GOT (visible) ----")
			term.Eprint(visibleWithLineNos(got))
			term.Eprintln("---- WANT (visible) ----")
			term.Eprint(visibleWithLineNos(want))
			term.Eprintln("========================================")
			term.Flush()
			t.Fatalf("%s: mismatch.\n--- got ---\n%s\n--- want ---\n%s", base, got, want)
		}

		got2, diags2 := FormatBytes(got)
		if len(diags2) != 0 {
			term.Eprintf(">> %s: parse diags on formatted:\n%+v\n", base, diags2)
			term.Flush()
			t.Fatalf("%s: parse diags (second pass): %+v", base, diags2)
		}
		if !bytes.Equal(got, got2) {
			term.Eprintln("========================================")
			term.Eprintf("%s: NOT IDEMPOTENT\n", base)
			term.Eprintln("---- PASS1 (visible) ----")
			term.Eprint(visibleWithLineNos(got))
			term.Eprintln("---- PASS2 (visible) ----")
			term.Eprint(visibleWithLineNos(got2))
			term.Eprintln("========================================")
			term.Flush()
			t.Fatalf("%s: not idempotent", base)
		}
	}
}

func visibleWithLineNos(b []byte) string {
	lines := bytes.Split(b, []byte("\n"))
	var out bytes.Buffer
	for i, ln := range lines {
		if i == len(lines)-1 && len(ln) == 0 {
			break
		}
		_, _ = fmt.Fprintf(&out, "%4d ", i+1) // ignore error
		ln = bytes.ReplaceAll(ln, []byte{'\t'}, []byte("⇥"))
		ln = bytes.ReplaceAll(ln, []byte(" "), []byte("·"))
		_, _ = out.Write(ln)
		_ = out.WriteByte('\n')
	}
	return out.String()
}
