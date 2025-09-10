package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/build"
	"github.com/desilang/desi/compiler/internal/check"
	cgen "github.com/desilang/desi/compiler/internal/codegen/c"
	"github.com/desilang/desi/compiler/internal/lexbridge"
	"github.com/desilang/desi/compiler/internal/parser"
	"github.com/desilang/desi/compiler/internal/term"
)

/* ---------- build (flags anywhere) ---------- */

type buildArgs struct {
	cc      string
	out     string
	file    string
	werr    bool // --Werror
	useDesi bool
	verbose bool
}

func parseBuildArgs(argv []string) (buildArgs, error) {
	var a buildArgs
	i := 0
	for i < len(argv) {
		s := argv[i]
		if s == "--" {
			i++
			break
		}
		switch {
		case strings.HasPrefix(s, "--cc="):
			a.cc = s[len("--cc="):]
			i++
			continue
		case s == "--cc":
			if i+1 >= len(argv) {
				return a, flag.ErrHelp
			}
			a.cc = argv[i+1]
			i += 2
			continue
		case strings.HasPrefix(s, "--out="):
			a.out = s[len("--out="):]
			i++
			continue
		case s == "--out":
			if i+1 >= len(argv) {
				return a, flag.ErrHelp
			}
			a.out = argv[i+1]
			i += 2
			continue
		case s == "--Werror" || s == "--werror":
			a.werr = true
			i++
			continue
		case s == "--use-desi-lexer":
			a.useDesi = true
			i++
			continue
		case s == "--verbose":
			a.verbose = true
			i++
			continue
		}
		if !strings.HasPrefix(s, "-") && a.file == "" {
			a.file = s
			i++
			continue
		}
		if strings.HasPrefix(s, "-") {
			return a, flag.ErrHelp
		}
		i++
	}
	for i < len(argv) && a.file == "" {
		if !strings.HasPrefix(argv[i], "-") {
			a.file = argv[i]
		}
		i++
	}
	if a.file == "" {
		return a, flag.ErrHelp
	}
	return a, nil
}

func cmdBuild(args []string) int {
	a, err := parseBuildArgs(args)
	if err != nil {
		term.Eprintln("usage: desic build [--cc=clang] [--out=name] [--Werror] [--use-desi-lexer] [--verbose] <entry.desi>")
		return 2
	}

	var (
		merged *ast.File
		perr   []error
	)

	if a.useDesi {
		// Desi-lexer path: provide a TokenSource loader to the resolver
		loader := func(absPath string) (parser.TokenSource, error) {
			src, err := lexbridge.NewSourceFromFileOpts(absPath, false /*keepTmp*/, a.verbose /*verbose*/)
			if err != nil {
				return nil, err
			}
			ts, ok := src.(parser.TokenSource)
			if !ok {
				return nil, fmt.Errorf("lexbridge: internal type mismatch (value does not satisfy parser.TokenSource)")
			}
			return ts, nil
		}
		merged, perr = build.ResolveAndParseWith(a.file, loader)
	} else {
		// Legacy Go-lexer path
		merged, perr = build.ResolveAndParse(a.file)
	}

	if len(perr) > 0 {
		for _, e := range perr {
			term.Eprintf("error: %v\n", e)
		}
		term.Eprintf("summary: %d error(s), %d warning(s)\n", len(perr), 0)
		return 1
	}

	// typecheck (errors block compile; warnings may block with --Werror)
	info, errs, warns := cgenCheckFileShim(merged)
	for _, w := range warns {
		term.Eprintf("warning: %s\n", w.String())
	}
	for _, e := range errs {
		term.Eprintf("error: %v\n", e)
	}
	if len(errs) > 0 || (a.werr && len(warns) > 0) {
		term.Eprintf("summary: %d error(s), %d warning(s)\n", len(errs), len(warns))
		return 1
	}

	// Emit C to gen/out — name based on entry file basename
	base := strings.TrimSuffix(filepath.Base(a.file), filepath.Ext(a.file))
	outDir := filepath.Join("gen", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		term.Eprintf("mkdir %s: %v\n", outDir, err)
		return 1
	}
	cpath := filepath.Join(outDir, base+".c")

	csrc := cgen.EmitFile(merged, info)
	if err := os.WriteFile(cpath, []byte(csrc), 0o644); err != nil {
		term.Eprintf("write %s: %v\n", cpath, err)
		return 1
	}
	term.Eprintf("wrote %s\n", cpath)

	// Optionally compile to gen/out/<out|basename>[.exe]
	if a.cc != "" {
		outName := a.out
		if outName == "" {
			outName = base
		}
		binPath := filepath.Join(outDir, outName)
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binPath), ".exe") {
			binPath += ".exe"
		}
		cmd := exec.Command(a.cc,
			cpath,
			filepath.Join("runtime", "c", "desi_std.c"),
			"-I", filepath.Join("runtime", "c"),
			// Silence MSVC deprecation spam for fopen on Windows:
			"-D_CRT_SECURE_NO_WARNINGS",
			"-o", binPath,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			term.Eprintf("cc failed: %v\n", err)
			return 1
		}
		term.Eprintf("built %s\n", binPath)
	}
	term.Eprintf("summary: %d error(s), %d warning(s)\n", 0, len(warns))
	return 0
}

// tiny local helper so we don't import check in multiple files
func cgenCheckFileShim(f *ast.File) (*check.Info, []error, []check.Warning) {
	return check.CheckFile(f)
}
