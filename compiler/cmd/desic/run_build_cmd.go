package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/project"
	"github.com/desilang/desi/compiler/internal/term"
)

func exit(code int) {
	term.Flush()
	os.Exit(code)
}

func init() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "init":
		exit(initCmd(os.Args[2:]))
	case "run":
		exit(runCmd(os.Args[2:]))
	case "build":
		exit(buildCmd(os.Args[2:]))
	case "test":
		exit(testCmd(os.Args[2:]))
	default:
		return
	}
}

// --------------------- init ---------------------

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

// --------------------- run/build/test ---------------------

func runCmd(argv []string) int {
	m, mp, ok := loadManifestOrFail("run")
	if !ok {
		return 2
	}
	entry := m.EntryPath()
	iroots := pickIRoots(argv, m)

	exe, _ := os.Executable()
	args := []string{"check"}
	// Preserve renderer flags and pass through explicitly.
	args = append(args, forwardRenderFlags(argv)...)
	if iroots != "" {
		args = append(args, "-I", iroots)
	}
	args = append(args, entry)

	cmd := exec.Command(exe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(mp)
	if err := cmd.Run(); err != nil {
		return exitCode(err)
	}
	term.Println("ok")
	return 0
}

func buildCmd(argv []string) int {
	m, mp, ok := loadManifestOrFail("build")
	if !ok {
		return 2
	}
	entry := m.EntryPath()
	outDir := m.OutDir()
	if outDir == "" {
		outDir = filepath.Join(filepath.Dir(mp), "build")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		term.Eprintln("build:", err)
		return 2
	}
	iroots := pickIRoots(argv, m)

	// Call self: `desic emit-ir ENTRY ...`
	exe, _ := os.Executable()
	args := []string{"emit-ir"}
	// Renderer flags pass-through (useful for diag consistency if the path errors)
	args = append(args, forwardRenderFlags(argv)...)
	if iroots != "" {
		args = append(args, "-I", iroots)
	}
	args = append(args, entry)

	var buf bytes.Buffer
	cmd := exec.Command(exe, args...)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(mp)
	if err := cmd.Run(); err != nil {
		return exitCode(err)
	}

	out := filepath.Join(outDir, safePkgName(m.Package.Name)+".ll")
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		term.Eprintln("build:", err)
		return 2
	}
	term.Println("wrote", out)
	return 0
}

func testCmd(argv []string) int {
	// Placeholder: run `go test ./...`
	cmd := exec.Command("go", "test", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitCode(err)
	}
	return 0
}

// --------------------- helpers ---------------------

func loadManifestOrFail(verb string) (project.Manifest, string, bool) {
	cwd, _ := os.Getwd()
	root, mp, ok := project.FindRoot(cwd)
	if !ok {
		term.Eprintln(verb+":", "no desi.mod found (run in a project directory)")
		return project.Manifest{}, "", false
	}
	m, diags := project.Load(mp)
	for _, d := range diags {
		d.RenderTTY(os.Stderr, diag.Theme{})
	}
	if len(diags) > 0 {
		return project.Manifest{}, "", false
	}
	_ = root
	return m, mp, true
}

func pickIRoots(argv []string, m project.Manifest) string {
	// CLI -I wins; otherwise manifest roots.
	if cli := parseIRootsArg(argv); cli != "" {
		return cli
	}
	rs := m.Roots()
	if len(rs) == 0 {
		return ""
	}
	sep := string(os.PathListSeparator) // ':' or ';'
	return strings.Join(rs, sep)
}

func parseIRootsArg(argv []string) string {
	iroots := ""
	sawSep := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if sawSep {
			break
		}
		switch {
		case a == "--":
			sawSep = true
		case a == "-I":
			if i+1 < len(argv) {
				iroots = argv[i+1]
				i++
			}
		case strings.HasPrefix(a, "-I="):
			iroots = strings.TrimPrefix(a, "-I=")
		}
	}
	return iroots
}

func forwardRenderFlags(argv []string) []string {
	out := []string{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case strings.HasPrefix(a, "--error-format="),
			a == "--error-format",
			strings.HasPrefix(a, "--color="),
			a == "--color":
			out = append(out, a)
			// if it was a split-arg form, grab the value too
			if (a == "--error-format" || a == "--color") && i+1 < len(argv) {
				out = append(out, argv[i+1])
				i++
			}
		}
	}
	return out
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(interface{ ExitStatus() int }); ok {
			return ws.ExitStatus()
		}
		// On Windows:
		if runtime.GOOS == "windows" {
			return int(ee.ExitCode())
		}
	}
	return 1
}

func safePkgName(name string) string {
	if name == "" {
		return "app"
	}
	// super simple sanitation
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
}
