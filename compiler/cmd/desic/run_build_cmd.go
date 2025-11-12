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
	// Syntax:
	//   desic init [NAME] [-p PATH|--path PATH] [-v VERSION|--version VERSION] [-e EDITION|--edition EDITION] [--force|-f]
	//
	// Behavior:
	// - If PATH provided: create there; package name = NAME if provided, else basename(PATH) (or CWD basename if PATH=".").
	// - If PATH not provided and NAME provided: create ./NAME; package name = NAME.
	// - If neither provided: target="."; package name = basename(CWD).
	force := false
	var name string
	var pathOpt string
	version := "0.1.0"
	edition := "2025"

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch a {
		case "-f", "--force":
			force = true
		case "-p", "--path":
			if i+1 >= len(argv) {
				term.Eprintln("init: missing value for", a)
				return 2
			}
			pathOpt = argv[i+1]
			i++
		case "-v", "--version":
			if i+1 >= len(argv) {
				term.Eprintln("init: missing value for", a)
				return 2
			}
			version = argv[i+1]
			i++
		case "-e", "--edition":
			if i+1 >= len(argv) {
				term.Eprintln("init: missing value for", a)
				return 2
			}
			edition = argv[i+1]
			i++
		default:
			// first non-flag token is NAME
			if strings.HasPrefix(a, "-") {
				term.Eprintln("init: unknown flag:", a)
				return 2
			}
			if name == "" {
				name = a
			} else {
				term.Eprintln("init: unexpected extra argument:", a)
				return 2
			}
		}
	}

	// Resolve target directory and package name
	var target string
	var pkgName string
	cwd, _ := os.Getwd()

	switch {
	case pathOpt != "":
		target = pathOpt
		if name != "" {
			pkgName = name
		} else {
			if target == "." {
				pkgName = filepath.Base(cwd)
			} else {
				pkgName = filepath.Base(target)
			}
		}
	case name != "":
		target = filepath.Join(cwd, name)
		pkgName = name
	default:
		target = "."
		pkgName = filepath.Base(cwd)
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	mp := filepath.Join(target, "desi.mod")
	if _, err := os.Stat(mp); err == nil && !force {
		term.Eprintln("init: desi.mod already exists (use --force to overwrite)")
		return 2
	}

	// Write desi.mod
	mod := "[package]\n" +
		"name    = " + quote(pkgName) + "\n" +
		"version = " + quote(version) + "\n" +
		"edition = " + quote(edition) + "\n" +
		"entry   = \"src/main.desi\"\n" +
		"roots   = [\"src\"]\n\n" +
		"[build]\n" +
		"mode    = \"debug\"\n" +
		"out_dir = \"build\"\n\n" +
		"[target]\n" +
		"triple  = \"native\"\n\n" +
		"[diagnostics]\n" +
		"error_format = \"human\"\n" +
		"color        = \"auto\"\n" +
		"max_errors   = \"100\"\n\n" +
		"[ffi]\n" +
		"libs   = []\n" +
		"search = []\n"
	if err := os.WriteFile(mp, []byte(mod), 0o644); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	// Write src/main.desi (no explicit prelude import; print is in prelude)
	srcDir := filepath.Join(target, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	mainDesi := "def main() -> int:\n" +
		"  print(\"Hello, Desi!\")\n" +
		"  0\n"
	if err := os.WriteFile(filepath.Join(srcDir, "main.desi"), []byte(mainDesi), 0o644); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	// Create tests/ dir
	if err := os.MkdirAll(filepath.Join(target, "tests"), 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	term.Println("initialized desi project at", target)
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

	// Call self: `desic emit-ir ENTRY` (do NOT pass -I; emit-ir path doesn’t honor it)
	exe, _ := os.Executable()
	args := []string{"emit-ir"}
	// Renderer flags pass-through (useful for diag consistency if the path errors)
	args = append(args, forwardRenderFlags(argv)...)
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

func quote(s string) string {
	// Minimal quoting for manifest strings
	if !strings.ContainsAny(s, " \t\"") {
		return `"` + s + `"` // still quote for consistency
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
