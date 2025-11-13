package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
)

// NOTE: This file is introduced in Task B with only `init` implemented.
// Task C will replace this file with a fuller version (run/build/test included).

func exit(code int) {
	term.Flush()
	os.Exit(code)
}

func init() {
	// Intercept only when actually running the binary, not during build/tests.
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "init":
		exit(initCmd(os.Args[2:]))
	default:
		return
	}
}

func initCmd(argv []string) int {
	// desic init [PATH] [--force|-f]
	force := false
	path := "."
	for _, a := range argv {
		switch a {
		case "--force", "-f":
			force = true
		default:
			if !strings.HasPrefix(a, "-") {
				path = a
			}
		}
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	mp := filepath.Join(path, "desi.mod")
	if _, err := os.Stat(mp); err == nil && !force {
		term.Eprintln("init: desi.mod already exists (use --force to overwrite)")
		return 2
	}

	// Write desi.mod
	mod := `[package]
name    = "hello-desi"
version = "0.1.0"
edition = "2025"
entry   = "src/main.desi"
roots   = ["src"]

[build]
mode    = "debug"
out_dir = "build"

[target]
triple  = "native"

[diagnostics]
error_format = "human"
color        = "auto"
max_errors   = "100"

[ffi]
libs   = []
search = []
`
	if err := os.WriteFile(mp, []byte(mod), 0o644); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	// Write src/main.desi
	srcDir := filepath.Join(path, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	mainDesi := "from prelude import print\n" +
		"def main() -> int:\n" +
		"  print(\"Hello, Desi!\")\n" +
		"  0\n"
	if err := os.WriteFile(filepath.Join(srcDir, "main.desi"), []byte(mainDesi), 0o644); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	// Create tests/ dir
	if err := os.MkdirAll(filepath.Join(path, "tests"), 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	term.Println("initialized desi project at", path)
	return 0
}
