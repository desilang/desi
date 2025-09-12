package main

import (
  "os"
  "os/exec"
  "path/filepath"
  "runtime"
  "testing"
)

// hasClang reports whether "clang" is on PATH.
func hasClang() bool {
  _, err := exec.LookPath("clang")
  return err == nil
}

// repoRoot returns the repo root by walking up from this file's location.
// This file lives at <root>/compiler/cmd/desic/integration_build_test.go
func repoRoot(t *testing.T) string {
  _, file, _, ok := runtime.Caller(0)
  if !ok {
    t.Fatalf("cannot determine caller path")
  }
  return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func pathExists(p string) bool {
  _, err := os.Stat(p)
  return err == nil
}

func TestBuild_WithDesiLexer_NoCC(t *testing.T) {
  if !hasClang() {
    t.Skip("clang not found; skipping lexbridge integration test")
  }

  root := repoRoot(t)

  // Paths expected by the bridge (relative to CWD). We will chdir to root
  // so these relative paths resolve correctly.
  entryRel := filepath.Join("examples", "parallel_demo.desi")
  devLexerRel := filepath.Join("examples", "compiler", "desi", "lexer.desi")

  // Ensure resources exist (checked relative to root).
  if !pathExists(filepath.Join(root, entryRel)) {
    t.Skipf("example not found: %s", filepath.Join(root, entryRel))
  }
  if !pathExists(filepath.Join(root, devLexerRel)) {
    t.Skipf("dev lexer not found: %s (bridge expects this path)", filepath.Join(root, devLexerRel))
  }

  // Temporarily chdir to repo root so bridge's relative paths work.
  cwd, err := os.Getwd()
  if err != nil {
    t.Fatalf("getwd: %v", err)
  }
  if err := os.Chdir(root); err != nil {
    t.Fatalf("chdir to repo root: %v", err)
  }
  defer func() { _ = os.Chdir(cwd) }()

  // Expect success building C (no cc link) via the Desi lexer bridge.
  code := cmdBuild([]string{"--use-desi-lexer", "--no-cc", entryRel})
  if code != 0 {
    t.Fatalf("desic build failed, exit=%d (cwd=%s, entry=%s)", code, root, entryRel)
  }
}
