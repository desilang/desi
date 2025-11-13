package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lower"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/project"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/term"
)

// init intercepts 'init', 'run', 'build', 'test' subcommands before main().
// This keeps the existing main.go unchanged for legacy flags.
func init() {
	if len(os.Args) < 2 {
		return
	}
	sub := os.Args[1]
	switch sub {
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

func exit(code int) {
	term.Flush()
	os.Exit(code)
}

// honorDiagDefaults loads desi.mod (if present) and sets global renderer defaults
// before parsing any subcommand-local flags. CLI flags will still override.
func honorDiagDefaults() {
	cwd, _ := os.Getwd()
	if _, mp, ok := project.FindRoot(cwd); ok {
		if m, diags := project.Load(mp); len(diags) == 0 {
			df := m.DiagDefaults()
			ef := df.ErrorFormat
			if ef == "" {
				ef = "human"
			}
			var cm diag.ColorMode
			switch df.Color {
			case "always":
				cm = diag.Always
			case "never":
				cm = diag.Never
			default:
				cm = diag.Auto
			}
			diag.SetGlobalRender(ef, cm)
		}
	}
}

func applyRenderOverrides(argv []string) {
	ef := ""
	col := ""
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case strings.HasPrefix(a, "--error-format="):
			ef = strings.ToLower(strings.TrimPrefix(a, "--error-format="))
		case a == "--error-format" && i+1 < len(argv):
			i++
			ef = strings.ToLower(argv[i])
		case strings.HasPrefix(a, "--color="):
			col = strings.ToLower(strings.TrimPrefix(a, "--color="))
		case a == "--color" && i+1 < len(argv):
			i++
			col = strings.ToLower(argv[i])
		}
	}
	if ef != "" || col != "" {
		if ef == "" {
			ef = "human"
		}
		var cm diag.ColorMode
		switch col {
		case "always":
			cm = diag.Always
		case "never":
			cm = diag.Never
		default:
			cm = diag.Auto
		}
		diag.SetGlobalRender(ef, cm)
	}
}

func initCmd(argv []string) int {
	honorDiagDefaults()
	// very small parser: desic init [PATH] [--force]
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
	// write desi.mod
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
	// write src/main.desi
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
	// create tests/ directory
	if err := os.MkdirAll(filepath.Join(path, "tests"), 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	term.Println("initialized desi project at", path)
	return 0
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

func resolveFrom(m project.Manifest, overrideRoots string) resolve.Loader {
	// CLI -I overrides manifest roots
	if overrideRoots != "" {
		return resolve.NewFSLoaderMulti(splitRoots(overrideRoots))
	}
	rs := m.Roots()
	if len(rs) == 0 {
		return resolve.NewMemLoader(nil)
	}
	return resolve.NewFSLoaderMulti(rs)
}

func runCmd(argv []string) int {
	honorDiagDefaults()
	applyRenderOverrides(argv)
	cwd, _ := os.Getwd()
	_, mp, ok := project.FindRoot(cwd)
	if !ok {
		term.Eprintln("run: no desi.mod found (run in a project directory)")
		return 2
	}
	m, diags := project.Load(mp)
	for _, d := range diags {
		d.RenderTTY(os.Stderr, diag.Theme{})
	}
	if len(diags) > 0 {
		return 2
	}
	entry := m.EntryPath()
	src, err := os.ReadFile(entry)
	if err != nil {
		term.Eprintln("run:", err)
		return 2
	}
	mod, pdiags := parse.ParseFile(entry, src)
	if len(pdiags) > 0 {
		for _, d := range pdiags {
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		return 2
	}
	// Type-check using manifest roots
	iroots := parseIRootsArg(argv)
	res := check.CheckWithLoader(mod, resolveFrom(m, iroots))
	if len(res.Diags) > 0 {
		for _, d := range res.Diags {
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		return 2
	}
	term.Println("ok")
	return 0
}

func buildCmd(argv []string) int {
	honorDiagDefaults()
	applyRenderOverrides(argv)
	cwd, _ := os.Getwd()
	_, mp, ok := project.FindRoot(cwd)
	if !ok {
		term.Eprintln("build: no desi.mod found (run in a project directory)")
		return 2
	}
	m, diags := project.Load(mp)
	for _, d := range diags {
		d.RenderTTY(os.Stderr, diag.Theme{})
	}
	if len(diags) > 0 {
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
	src, err := os.ReadFile(entry)
	if err != nil {
		term.Eprintln("build:", err)
		return 2
	}
	// Parse + pre-desugar like emit-ir
	mod, pdiags := parse.ParseFile(entry, src)
	if len(pdiags) > 0 {
		for _, d := range pdiags {
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		return 2
	}
	check.DesugarPrecheck(mod)
	iroots := parseIRootsArg(argv)
	_ = iroots // reserved for future; loader not needed for lowering
	hm := lower.LowerModuleFromSource(mod, src)
	if hm == nil || len(hm.Funcs) == 0 {
		term.Eprintln("build:", filepath.Base(entry)+": no functions to lower")
		return 2
	}
	lm := llvm.NewModule(filepath.Base(entry))
	// inject textual sigs
	injectUserFuncSigs(mod)
	for _, f := range hm.Funcs {
		lm.EmitFunc(f)
	}
	var buf bytes.Buffer
	buf.WriteString(lm.IR())
	out := filepath.Join(outDir, m.Package.Name+".ll")
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		term.Eprintln("build:", err)
		return 2
	}
	term.Println("wrote", out)
	return 0
}

func testCmd(argv []string) int {
	honorDiagDefaults()
	applyRenderOverrides(argv)
	cmd := exec.Command("go", "test", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// propagate exit status 1
		return 1
	}
	return 0
}
