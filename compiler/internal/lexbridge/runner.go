package lexbridge

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/desilang/desi/compiler/internal/build"
	"github.com/desilang/desi/compiler/internal/check"
	cgen "github.com/desilang/desi/compiler/internal/codegen/c"
	"github.com/desilang/desi/compiler/internal/term"
)

// BuildAndRunRaw builds the temp wrapper, runs it, and returns the raw token stream
// ("KIND|TEXT|LINE|COL\n" ...). If keepTmp=false, it deletes gen/tmp/lexbridge afterward.
// On Windows we emit/rerun lexbridge_run.exe and execute it via absolute path.
func BuildAndRunRaw(filePath string, keepTmp bool) (string, error) {
	// Read user source
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Temp tree
	tmpRoot, tmpWrapper, tmpLexerPath, err := BuildTempTree()
	if err != nil {
		return "", err
	}

	// Mirror dev lexer into temp import tree
	srcLexer := filepath.Join("examples", "compiler", "desi", "lexer.desi")
	if err := CopyFile(srcLexer, tmpLexerPath); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", err
	}

	// Wrapper
	srcLiteral := EscapeForDesiString(string(data))
	wrapper := strings.Join([]string{
		"import compiler.desi.lexer",
		"",
		"def main() -> int:",
		"  let src = " + srcLiteral,
		"  let toks = lex_tokens(src)",
		"  io.println(toks)",
		"  return 0",
		"",
	}, "\n")

	if err := os.WriteFile(tmpWrapper, []byte(wrapper), 0o644); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", err
	}

	// Build wrapper using existing pipeline (emit C + compile with clang)
	binPath, rc := buildWrapper(tmpWrapper, "lexbridge_run")
	if rc != 0 {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", ErrBuildFailed
	}

	// Execute and capture stdout using absolute path (no PATH lookups)
	absBin, _ := filepath.Abs(binPath)
	cmd := exec.Command(absBin)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", err
	}

	if !keepTmp {
		_ = os.RemoveAll(tmpRoot)
	}
	return out.String(), nil
}

// ErrBuildFailed is returned when C compilation failed via buildWrapper path.
var ErrBuildFailed = exec.ErrNotFound // sentinel; non-nil indicates failure

// buildWrapper is a tiny local copy of the critical steps from main.cmdBuild, so
// this package can be used by both CLI and (later) build/parse paths.
// It returns the final executable path (with .exe on Windows) and a return code.
func buildWrapper(entry string, outName string) (string, int) {
	// Resolve+parse (multi-file)
	merged, perr := build.ResolveAndParse(entry)
	if len(perr) > 0 {
		for _, e := range perr {
			term.Eprintf("error: %v\n", e)
		}
		term.Eprintf("summary: %d error(s), %d warning(s)\n", len(perr), 0)
		return "", 1
	}

	// Typecheck
	info, errs, warns := check.CheckFile(merged)
	for _, w := range warns {
		term.Eprintf("warning: %s\n", w.String())
	}
	for _, e := range errs {
		term.Eprintf("error: %v\n", e)
	}
	if len(errs) > 0 {
		term.Eprintf("summary: %d error(s), %d warning(s)\n", len(errs), len(warns))
		return "", 1
	}

	// Emit C
	base := "main" // wrapper is named main.desi; we'll just stick to main.c
	outDir := filepath.Join("gen", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		term.Eprintf("mkdir %s: %v\n", outDir, err)
		return "", 1
	}
	cpath := filepath.Join(outDir, base+".c")
	csrc := cgen.EmitFile(merged, info)
	if err := os.WriteFile(cpath, []byte(csrc), 0o644); err != nil {
		term.Eprintf("write %s: %v\n", cpath, err)
		return "", 1
	}
	term.Eprintf("wrote %s\n", cpath)

	// Compute final binary path (+.exe on Windows)
	binPath := filepath.Join(outDir, outName)
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binPath), ".exe") {
		binPath += ".exe"
	}

	// Compile with clang (silence fopen deprecation on Windows)
	args := []string{
		cpath,
		filepath.Join("runtime", "c", "desi_std.c"),
		"-I", filepath.Join("runtime", "c"),
		"-o", binPath,
	}
	if runtime.GOOS == "windows" {
		args = append([]string{"-D_CRT_SECURE_NO_WARNINGS"}, args...)
	}
	cmd := exec.Command("clang", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		term.Eprintf("cc failed: %v\n", err)
		return "", 1
	}
	term.Eprintf("built %s\n", binPath)
	term.Eprintf("summary: %d error(s), %d warning(s)\n", 0, len(warns))
	return binPath, 0
}
