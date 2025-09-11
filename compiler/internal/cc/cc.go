package cc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Options struct {
	// CSource is the path to the generated C file (e.g. gen/out/parallel_demo.c)
	CSource string

	// Out is the desired output executable path (e.g. gen/out/parallel_demo or .exe on Windows).
	// If empty, we'll derive it from CSource by dropping the extension.
	Out string

	// RuntimeDir is the path to the repo's runtime/c folder that contains desi_std.h and desi_std.c.
	// If empty, we will auto-detect by walking up from CSource and working directory.
	RuntimeDir string

	// CCBin is an optional explicit compiler (e.g. "clang", "gcc", "cl").
	// If empty, we will detect (clang > gcc on Unix; clang > cl > gcc on Windows).
	CCBin string

	// ExtraArgs lets callers pass additional flags if desired (kept minimal by default).
	ExtraArgs []string

	// Disable actually invoking the C compiler; if true, we only validate & return success.
	DryRun bool
}

// Compile compiles the generated C file together with the runtime library,
// adding the correct include path. It picks a sensible default compiler per-OS
// and requires no user flags.
//
// On success, the output executable is written at opts.Out (or derived from opts.CSource).
func Compile(opts Options) error {
	if opts.CSource == "" {
		return errors.New("cc: CSource must be set")
	}
	srcAbs, err := filepath.Abs(opts.CSource)
	if err != nil {
		return fmt.Errorf("cc: resolve CSource: %w", err)
	}
	if _, err := os.Stat(srcAbs); err != nil {
		return fmt.Errorf("cc: source does not exist: %s", srcAbs)
	}

	out := opts.Out
	if out == "" {
		out = dropExt(srcAbs)
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(out), ".exe") {
		out = out + ".exe"
	}
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return fmt.Errorf("cc: resolve Out: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outAbs), 0o755); err != nil {
		return fmt.Errorf("cc: create out dir: %w", err)
	}

	rtDir := opts.RuntimeDir
	if rtDir == "" {
		rtDir, err = findRuntimeDir(srcAbs)
		if err != nil {
			return err
		}
	}
	rtAbs, err := filepath.Abs(rtDir)
	if err != nil {
		return fmt.Errorf("cc: resolve RuntimeDir: %w", err)
	}
	if _, err := os.Stat(filepath.Join(rtAbs, "desi_std.h")); err != nil {
		return fmt.Errorf("cc: missing desi_std.h in runtime dir: %s", rtAbs)
	}
	if _, err := os.Stat(filepath.Join(rtAbs, "desi_std.c")); err != nil {
		return fmt.Errorf("cc: missing desi_std.c in runtime dir: %s", rtAbs)
	}

	cc := opts.CCBin
	if cc == "" {
		cc, err = pickCompiler()
		if err != nil {
			return err
		}
	}

	args := constructArgs(cc, srcAbs, outAbs, rtAbs, opts.ExtraArgs)
	if opts.DryRun {
		return nil
	}

	cmd := exec.Command(cc, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cc: compilation failed: %w", err)
	}
	return nil
}

func dropExt(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// findRuntimeDir tries a few reasonable locations:
//  1. from CSource: ../../runtime/c (repo-root-ish)
//  2. from current working dir: ./runtime/c
//  3. walk up from CSource directory until we find runtime/c/desi_std.h (max 6 levels)
func findRuntimeDir(cSourceAbs string) (string, error) {
	// candidate 1: repo-root relative to gen/out/<file>.c
	start := filepath.Dir(filepath.Dir(filepath.Dir(cSourceAbs))) // go up from gen/out/<name>.c to repo-ish
	cand1 := filepath.Join(start, "runtime", "c")
	if fileExists(filepath.Join(cand1, "desi_std.h")) {
		return cand1, nil
	}

	// candidate 2: cwd-relative
	cwd, _ := os.Getwd()
	cand2 := filepath.Join(cwd, "runtime", "c")
	if fileExists(filepath.Join(cand2, "desi_std.h")) {
		return cand2, nil
	}

	// candidate 3: walk up from CSource dir
	dir := filepath.Dir(cSourceAbs)
	for i := 0; i < 6; i++ {
		cand := filepath.Join(dir, "runtime", "c")
		if fileExists(filepath.Join(cand, "desi_std.h")) {
			return cand, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}

	return "", fmt.Errorf("cc: could not locate runtime/c (desi_std.h) starting from %s", cSourceAbs)
}

func pickCompiler() (string, error) {
	// Allow env override
	if v := os.Getenv("DESI_CC"); v != "" {
		if _, err := exec.LookPath(v); err == nil {
			return v, nil
		}
	}

	if runtime.GOOS == "windows" {
		// Prefer clang, then cl, then gcc
		if hasCmd("clang") {
			return "clang", nil
		}
		if hasCmd("cl") {
			return "cl", nil
		}
		if hasCmd("gcc") {
			return "gcc", nil
		}
		return "", errors.New("cc: no compiler found (tried clang, cl, gcc)")
	}

	// POSIX: prefer clang then gcc
	if hasCmd("clang") {
		return "clang", nil
	}
	if hasCmd("gcc") {
		return "gcc", nil
	}
	// Some systems alias cc -> clang or gcc
	if hasCmd("cc") {
		return "cc", nil
	}
	return "", errors.New("cc: no compiler found (need clang or gcc)")
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func constructArgs(cc, srcAbs, outAbs, rtAbs string, extra []string) []string {
	isMSVC := strings.EqualFold(cc, "cl")

	if isMSVC {
		// cl /nologo src desi_std.c /I runtime\c /Fe:out.exe
		args := []string{
			"/nologo",
			srcAbs,
			filepath.Join(rtAbs, "desi_std.c"),
			"/I", rtAbs,
			"/Fe:" + outAbs,
		}
		return append(args, extra...)
	}

	// gcc/clang: cc src desi_std.c -I runtime/c -o out
	args := []string{
		srcAbs,
		filepath.Join(rtAbs, "desi_std.c"),
		"-I", rtAbs,
		"-o", outAbs,
	}
	return append(args, extra...)
}
