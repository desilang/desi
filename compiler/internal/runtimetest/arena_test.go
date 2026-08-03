// Package runtimetest drives the runtime's own C test programs.
//
// The arena is one of the few pieces of the runtime with invariants that are
// easier to state in C than to reach through generated code — nothing in a
// Desi program can observe a chunk list directly. The properties live in
// compiler/runtime/tests/*.c and this test compiles and runs them, so they go
// through CI with everything else rather than needing their own job.
package runtimetest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestArenaMarkRewind(t *testing.T) {
	cc, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang not on PATH; skipping runtime C test")
	}

	root := filepath.Join("..", "..", "..")
	src := filepath.Join(root, "compiler", "runtime", "tests", "arena_mark.c")
	impl := filepath.Join(root, "compiler", "runtime", "arena.c")

	dir := t.TempDir()

	// Built both ways. The poison build is what the example suite is run under
	// when the arena changes, and it only means anything if the poisoning
	// itself is working, which the second variant checks.
	for _, variant := range []struct {
		name  string
		flags []string
	}{
		{"plain", nil},
		{"poison", []string{"-DDESI_ARENA_POISON"}},
	} {
		t.Run(variant.name, func(t *testing.T) {
			bin := filepath.Join(dir, "arena_mark_"+variant.name)
			if runtime.GOOS == "windows" {
				bin += ".exe"
			}

			args := append([]string{"-O1", "-Wall"}, variant.flags...)
			args = append(args, "-o", bin, src, impl)
			if out, err := exec.Command(cc, args...).CombinedOutput(); err != nil {
				t.Fatalf("compiling the arena test failed: %v\n%s", err, out)
			}
			if _, err := os.Stat(bin); err != nil {
				t.Fatalf("test binary was not produced: %v", err)
			}

			out, err := exec.Command(bin).CombinedOutput()
			t.Logf("%s", out)
			if err != nil {
				t.Fatalf("arena properties do not hold: %v", err)
			}
		})
	}
}
