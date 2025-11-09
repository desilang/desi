package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/format"
)

func TestDesifmt_ReadsFromStdinWithDash(t *testing.T) {
	src := "def f() -> int:\n\t1+2  # sum\n\t0\n"
	wantBytes, diags := format.FormatBytes([]byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags computing golden: %+v", diags)
	}

	cmd := exec.Command("go", "run", ".", "-")
	cmd.Dir = "." // this package directory
	cmd.Stdin = bytes.NewReader([]byte(src))

	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		t.Fatalf("desifmt (stdin) exit=%v stderr=%s", err, errb.String())
	}

	got := out.Bytes()
	if !bytes.Equal(got, wantBytes) {
		t.Fatalf("stdin formatting mismatch\n--- got ---\n%s\n--- want ---\n%s", got, wantBytes)
	}
}

func TestDesifmt_ListModePrintsPathsOnStdout(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "a.desi")
	src := "def main() -> int:\n\t1+2  # sum\n\t0\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "-l", path)
	cmd.Dir = "."
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		t.Fatalf("-l exit=%v stderr=%s", err, errb.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	found := false
	for _, ln := range lines {
		if strings.TrimSpace(ln) == path {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -l to list %s on stdout; got %q", path, out.String())
	}
}

func TestDesifmt_WriteModeUpdatesFile(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "b.desi")
	src := "def main() -> int:\n\t1+2  # sum\n\t0\n"
	wantBytes, diags := format.FormatBytes([]byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags computing golden: %+v", diags)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// -w should rewrite the file; -q suppresses "wrote ..." chatter
	cmd := exec.Command("go", "run", ".", "-w", "-q", path)
	cmd.Dir = "."
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		t.Fatalf("-w exit=%v stderr=%s", err, errb.String())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Fatalf("file content after -w mismatch\n--- got ---\n%s\n--- want ---\n%s", got, wantBytes)
	}
}
