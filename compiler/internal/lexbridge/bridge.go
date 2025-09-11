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

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	cgen "github.com/desilang/desi/compiler/internal/codegen/c"
	"github.com/desilang/desi/compiler/internal/parser"
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
	srcLiteral := escapeDesiString(string(data))
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

	// 4) Resolve+parse+typecheck wrapper (no noisy prints) — local resolver to avoid build import cycles
	merged, perr := resolveAndParseLocal(wrapperPath)
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

// resolveAndParseLocal mirrors Stage-0 import rules without importing build:
//   - "foo.bar" -> "<root>/foo/bar.desi"
//   - ignore imports starting with "std."
//   - detect cycles; skip duplicates
//
// It merges entry decls first, then deps.
func resolveAndParseLocal(entryPath string) (*ast.File, []error) {
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
	}
	rootDir := filepath.Dir(entryAbs)

	type unit struct {
		path string
		file *ast.File
	}
	var (
		errs   []error
		seen   = map[string]bool{}
		stack  = []string{}
		result = []*unit{}
	)

	var load func(absPath string)
	load = func(absPath string) {
		if seen[absPath] {
			return
		}
		// cycle check
		for _, on := range stack {
			if on == absPath {
				errs = append(errs, fmt.Errorf("import cycle detected involving %s", rel(rootDir, absPath)))
				return
			}
		}
		stack = append(stack, absPath)
		defer func() { stack = stack[:len(stack)-1] }()

		data, rerr := os.ReadFile(absPath)
		if rerr != nil {
			errs = append(errs, fmt.Errorf("read %s: %v", rel(rootDir, absPath), rerr))
			return
		}
		p := parser.New(string(data))
		f, perr := p.ParseFile()
		if perr != nil {
			errs = append(errs, fmt.Errorf("parse %s: %v", rel(rootDir, absPath), perr))
			return
		}

		// resolve imports (Stage-0 rules)
		for _, imp := range f.Imports {
			path := imp.Path
			if strings.HasPrefix(path, "std.") {
				continue
			}
			relPath := strings.ReplaceAll(path, ".", string(filepath.Separator)) + ".desi"
			target := filepath.Join(rootDir, relPath)
			if _, statErr := os.Stat(target); statErr != nil {
				errs = append(errs, fmt.Errorf("import %q → %s not found (from %s)",
					path, rel(rootDir, target), rel(rootDir, absPath)))
				continue
			}
			load(mustAbs(target))
		}

		result = append(result, &unit{path: absPath, file: f})
		seen[absPath] = true
	}

	load(entryAbs)

	if len(errs) > 0 {
		return nil, errs
	}

	var merged ast.File
	merged.Pkg = nil
	merged.Imports = nil
	merged.Decls = nil

	// entry first
	for _, u := range result {
		if same(u.path, entryAbs) {
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}
	// then deps
	for _, u := range result {
		if !same(u.path, entryAbs) {
			merged.Decls = append(merged.Decls, u.file.Decls...)
		}
	}

	return &merged, nil
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

// --- small local helpers (avoid importing build) ---

func mustAbs(p string) string {
	a, _ := filepath.Abs(p)
	return a
}
func same(a, b string) bool {
	aa, _ := filepath.EvalSymlinks(a)
	bb, _ := filepath.EvalSymlinks(b)
	if aa == "" {
		aa = a
	}
	if bb == "" {
		bb = b
	}
	return aa == bb
}
func rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return r
}

// escapeDesiString converts raw text into a double-quoted Desi string literal.
func escapeDesiString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				o1 := ((r >> 6) & 7) + '0'
				o2 := ((r >> 3) & 7) + '0'
				o3 := (r & 7) + '0'
				b.WriteByte('\\')
				b.WriteByte(byte(o1))
				b.WriteByte(byte(o2))
				b.WriteByte(byte(o3))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
