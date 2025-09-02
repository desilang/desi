package lexbridge

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/desilang/desi/compiler/internal/build"
	"github.com/desilang/desi/compiler/internal/check"
	cgen "github.com/desilang/desi/compiler/internal/codegen/c"
)

// BuildAndRunRaw compiles a tiny wrapper that calls compiler.desi.lexer::lex_tokens
// on the given source file, then runs the produced binary and returns its stdout.
// If verbose=false, all Clang output (warnings/errors) is suppressed; on failure,
// we return a concise error suggesting --verbose. No stale-binary fallbacks.
func BuildAndRunRaw(srcFile string, keepTmp bool, verbose bool) (string, error) {
	// 1) Read input source
	data, err := os.ReadFile(srcFile)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", srcFile, err)
	}

	// 2) Create temp tree and mirror dev lexer import:
	//    gen/tmp/lexbridge/
	//      main.desi
	//      compiler/desi/lexer.desi   (copied from examples/compiler/desi/lexer.desi)
	tmpRoot := filepath.Join("gen", "tmp", "lexbridge")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", tmpRoot, err)
	}
	if err := mirrorDevLexer(tmpRoot); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", err
	}

	// 3) Write wrapper
	wrapperPath := filepath.Join(tmpRoot, "main.desi")
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
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o644); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", fmt.Errorf("write wrapper: %w", err)
	}

	// 4) Resolve+parse+typecheck wrapper (no noisy prints)
	merged, perr := build.ResolveAndParse(wrapperPath)
	if len(perr) > 0 {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		var b strings.Builder
		for _, e := range perr {
			_, _ = fmt.Fprintf(&b, "error: %v\n", e)
		}
		return "", fmt.Errorf("resolve/parse wrapper failed:\n%s", b.String())
	}

	info, errs, _ := check.CheckFile(merged)
	if len(errs) > 0 {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		var b strings.Builder
		for _, e := range errs {
			_, _ = fmt.Fprintf(&b, "error: %v\n", e)
		}
		return "", fmt.Errorf("typecheck wrapper failed:\n%s", b.String())
	}

	// 5) Emit C
	outDir := filepath.Join("gen", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	cpath := filepath.Join(outDir, "main.c")
	csrc := cgen.EmitFile(merged, info)
	if err := os.WriteFile(cpath, []byte(csrc), 0o644); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		return "", fmt.Errorf("write %s: %w", cpath, err)
	}

	// 6) clang compile → gen/out/lexbridge_run[.exe]
	binPath := filepath.Join(outDir, "lexbridge_run")
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binPath), ".exe") {
		binPath += ".exe"
	}
	clang := "clang"
	cc := exec.Command(clang,
		cpath,
		filepath.Join("runtime", "c", "desi_std.c"),
		"-I", filepath.Join("runtime", "c"),
		"-D_CRT_SECURE_NO_WARNINGS", // hush fopen warnings on Windows
		"-o", binPath,
	)
	if verbose {
		cc.Stdout = os.Stdout
		cc.Stderr = os.Stderr
	} else {
		var sink bytes.Buffer
		cc.Stdout = &sink
		cc.Stderr = &sink
	}
	if err := cc.Run(); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		if verbose {
			return "", fmt.Errorf("cc failed: %w", err)
		}
		return "", fmt.Errorf("cc failed; re-run with --verbose to see compiler output")
	}

	// 7) Run bridge binary; capture stdout (tokens), silence stderr unless verbose
	absBin, _ := filepath.Abs(binPath)
	run := exec.Command(absBin)
	var out bytes.Buffer
	run.Stdout = &out
	if verbose {
		run.Stderr = os.Stderr
	} else {
		run.Stderr = io.Discard
	}
	if err := run.Run(); err != nil {
		if !keepTmp {
			_ = os.RemoveAll(tmpRoot)
		}
		if verbose {
			return "", fmt.Errorf("run wrapper: %w", err)
		}
		return "", fmt.Errorf("run wrapper failed; re-run with --verbose for details")
	}

	// 8) Cleanup temp sources if requested
	if !keepTmp {
		_ = os.RemoveAll(tmpRoot)
	}

	return out.String(), nil
}

// mirrorDevLexer copies examples/compiler/desi/lexer.desi under tmpRoot/compiler/desi/lexer.desi
func mirrorDevLexer(tmpRoot string) error {
	src := filepath.Join("examples", "compiler", "desi", "lexer.desi")
	dstDir := filepath.Join(tmpRoot, "compiler", "desi")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstDir, err)
	}
	dst := filepath.Join(dstDir, "lexer.desi")
	in, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, in, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
