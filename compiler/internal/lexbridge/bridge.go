package lexbridge

import (
  "bytes"
  "crypto/sha256"
  "encoding/hex"
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

// BuildAndRunRaw compiles (or reuses a cached) tiny wrapper that calls
// compiler.desi.lexer::lex_tokens on the given source file, then runs the
// produced binary and returns its stdout.
//
// Caching:
//   - We compute a hash over: (wrapper content) + (dev lexer content) +
//     (runtime/c sources) + (bridge version tag) + (GOOS/GOARCH).
//   - Binary is cached at gen/tmp/lexbridge/cache/<hash>/lexbridge_run[.exe].
//   - If cached binary exists, we skip rebuilding.
//
// keepTmp: previously controlled tmp cleanup; with caching, we keep cache
// directories. On build failure and keepTmp=false, we still clean the work dir.
// verbose: prints clang output and wrapper runtime stderr if true.
func BuildAndRunRaw(srcFile string, keepTmp bool, verbose bool) (string, error) {
  // 1) Read user source
  userSrc, err := os.ReadFile(srcFile)
  if err != nil {
    return "", fmt.Errorf("read %s: %w", srcFile, err)
  }

  // 2) Locate dev lexer (examples/compiler/desi/lexer.desi)
  repoRelDevLexer := filepath.Join("examples", "compiler", "desi", "lexer.desi")
  devLexSrc, err := os.ReadFile(repoRelDevLexer)
  if err != nil {
    return "", fmt.Errorf("load parallel_demo.desi: read %s: %w", repoRelDevLexer, err)
  }

  // 3) Runtime C sources — include these in the cache signature so a change busts cache
  rtDir := filepath.Join("runtime", "c")
  rtC := filepath.Join(rtDir, "desi_std.c")
  rtH := filepath.Join(rtDir, "desi_std.h")
  rtCSrc, _ := os.ReadFile(rtC)
  rtHSrc, _ := os.ReadFile(rtH)

  // 4) Build wrapper source (lives under workDir). The wrapper imports the mirrored dev lexer.
  wrapper := buildWrapper(string(userSrc))

  // 5) Compute cache key
  sig := sha256Sum(
    "bridge-v2",       // tag to invalidate older cache strategies
    runtime.GOOS,      // cross-OS differs
    runtime.GOARCH,    // cross-arch differs
    string(wrapper),   // wrapper content
    string(devLexSrc), // lexer content
    string(rtCSrc),    // runtime C
    string(rtHSrc),    // runtime H
  )
  cacheRoot := filepath.Join("gen", "tmp", "lexbridge", "cache", sig)
  binPath := filepath.Join(cacheRoot, "lexbridge_run")
  if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binPath), ".exe") {
    binPath += ".exe"
  }

  // If cached binary exists, just run it.
  if fileExists(binPath) {
    return runBridge(binPath, verbose)
  }

  // 6) Build work dir (same as cache dir; we preserve it for caching)
  workDir := cacheRoot
  if err := os.MkdirAll(workDir, 0o755); err != nil {
    return "", fmt.Errorf("mkdir %s: %w", workDir, err)
  }

  // Mirror dev lexer under workDir/compiler/desi/lexer.desi
  if err := mirrorDevLexerInto(workDir, devLexSrc); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(workDir)
    }
    return "", err
  }

  // Write wrapper
  wrapperPath := filepath.Join(workDir, "main.desi")
  if err := os.WriteFile(wrapperPath, wrapper, 0o644); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(workDir)
    }
    return "", fmt.Errorf("write wrapper: %w", err)
  }

  // 7) Resolve+parse+typecheck wrapper locally (Stage-0 import rules)
  merged, perr := resolveAndParseLocal(workDir, wrapperPath)
  if len(perr) > 0 {
    if !keepTmp {
      _ = os.RemoveAll(workDir)
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
      _ = os.RemoveAll(workDir)
    }
    var b strings.Builder
    for _, e := range errs {
      _, _ = fmt.Fprintf(&b, "error: %v\n", e)
    }
    return "", fmt.Errorf("typecheck wrapper failed:\n%s", b.String())
  }

  // 8) Emit C under workDir (do not pollute gen/out)
  cpath := filepath.Join(workDir, "main.c")
  csrc := cgen.EmitFile(merged, info)
  if err := os.WriteFile(cpath, []byte(csrc), 0o644); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(workDir)
    }
    return "", fmt.Errorf("write %s: %w", cpath, err)
  }

  // 9) Compile bridge binary with clang → binPath
  if err := compileBridge(cpath, rtDir, binPath, verbose); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(workDir)
    }
    if verbose {
      return "", fmt.Errorf("cc failed: %w", err)
    }
    return "", fmt.Errorf("cc failed; re-run with --bridge-verbose to see compiler output")
  }

  // 10) Run cached/new binary; capture stdout (tokens)
  return runBridge(binPath, verbose)
}

/* ---------- helpers ---------- */

func sha256Sum(parts ...string) string {
  h := sha256.New()
  for _, p := range parts {
    _, _ = io.WriteString(h, p)
    _, _ = io.WriteString(h, "\x00")
  }
  return hex.EncodeToString(h.Sum(nil))
}

func compileBridge(cpath, rtDir, outBin string, verbose bool) error {
  clang := "clang"
  args := []string{
    cpath,
    filepath.Join(rtDir, "desi_std.c"),
    "-I", rtDir,
    "-D_CRT_SECURE_NO_WARNINGS", // hush fopen warnings on Windows
    "-o", outBin,
  }
  cc := exec.Command(clang, args...)
  if verbose {
    cc.Stdout = os.Stdout
    cc.Stderr = os.Stderr
  } else {
    var sink bytes.Buffer
    cc.Stdout = &sink
    cc.Stderr = &sink
  }
  return cc.Run()
}

func runBridge(binPath string, verbose bool) (string, error) {
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
    if verbose {
      return "", fmt.Errorf("run wrapper: %w", err)
    }
    return "", fmt.Errorf("run wrapper failed; re-run with --bridge-verbose for details")
  }
  return out.String(), nil
}

func mirrorDevLexerInto(root string, devLexSrc []byte) error {
  dstDir := filepath.Join(root, "compiler", "desi")
  if err := os.MkdirAll(dstDir, 0o755); err != nil {
    return fmt.Errorf("mkdir %s: %w", dstDir, err)
  }
  dst := filepath.Join(dstDir, "lexer.desi")
  if err := os.WriteFile(dst, devLexSrc, 0o644); err != nil {
    return fmt.Errorf("write %s: %w", dst, err)
  }
  return nil
}

func buildWrapper(userSrc string) []byte {
  srcLiteral := escapeDesiString(userSrc)
  var b strings.Builder
  b.WriteString("import compiler.desi.lexer\n\n")
  b.WriteString("def main() -> int:\n")
  b.WriteString("  let src = ")
  b.WriteString(srcLiteral)
  b.WriteString("\n")
  b.WriteString("  let toks = lex_tokens(src)\n")
  b.WriteString("  io.println(toks)\n")
  b.WriteString("  return 0\n")
  b.WriteString("\n")
  return []byte(b.String())
}

// escapeDesiString returns a double-quoted literal for Desi with C-like escapes.
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

/* ---------- tiny local resolver (Stage-0) ---------- */

// resolveAndParseLocal implements a minimal import resolver for the wrapper tree.
// Rules (Stage-0):
//   - import "a.b.c" resolves to "<root>/a/b/c.desi"
//   - "std.*" imports are ignored (runtime-provided)
//   - cycles are detected
func resolveAndParseLocal(rootDir, entryPath string) (*ast.File, []error) {
  entryAbs, err := filepath.Abs(entryPath)
  if err != nil {
    return nil, []error{fmt.Errorf("abs(%s): %v", entryPath, err)}
  }

  type unit struct {
    path string
    file *ast.File
  }
  var (
    errs   []error
    seen   = map[string]bool{}
    stack  []string
    result []*unit
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

    data, err := os.ReadFile(absPath)
    if err != nil {
      errs = append(errs, fmt.Errorf("read %s: %v", rel(rootDir, absPath), err))
      return
    }
    p := parser.New(string(data))
    f, perr := p.ParseFile()
    if perr != nil {
      errs = append(errs, fmt.Errorf("parse %s: %v", rel(rootDir, absPath), perr))
      return
    }

    // resolve imports
    for _, imp := range f.Imports {
      path := imp.Path
      if strings.HasPrefix(path, "std.") {
        continue
      }
      relPath := strings.ReplaceAll(path, ".", string(filepath.Separator)) + ".desi"
      target := filepath.Join(rootDir, relPath)
      if !fileExists(target) {
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

  // Merge: entry first, then others
  var merged ast.File
  for _, u := range result {
    if same(u.path, entryAbs) {
      merged.Decls = append(merged.Decls, u.file.Decls...)
    }
  }
  for _, u := range result {
    if !same(u.path, entryAbs) {
      merged.Decls = append(merged.Decls, u.file.Decls...)
    }
  }
  return &merged, nil
}

func fileExists(p string) bool {
  _, err := os.Stat(p)
  return err == nil
}
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
