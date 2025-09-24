package parsebridge

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

  "github.com/desilang/desi/compiler/internal/term"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/check"
  cgen "github.com/desilang/desi/compiler/internal/codegen/c"
  "github.com/desilang/desi/compiler/internal/parser"
)

// ----- errors -----

type ParserSourcesMissingError struct {
  Path string
}

func (e ParserSourcesMissingError) Error() string {
  return "Desi parser source not found at " + e.Path
}

// ----- public entrypoint -----

// BuildAndRunJSON compiles (and caches) a tiny wrapper that calls
// compiler.desi.parser::parse_to_json(path) and returns its stdout (AST JSON).
// Requires a dev parser at examples/compiler/desi/parser.desi.
// We also mirror examples/compiler/desi/lexer.desi so the parser can import it.
func BuildAndRunJSON(entryFile string, keepTmp, verbose bool) ([]byte, error) {
  entryAbs, err := filepath.Abs(entryFile)
  if err != nil {
    return nil, fmt.Errorf("abs(%s): %v", entryFile, err)
  }

  // dev sources
  repoRelParser := filepath.Join("examples", "compiler", "desi", "parser.desi")
  devParserSrc, err := os.ReadFile(repoRelParser)
  if err != nil {
    return nil, ParserSourcesMissingError{Path: repoRelParser}
  }
  repoRelLexer := filepath.Join("examples", "compiler", "desi", "lexer.desi")
  devLexerSrc, _ := os.ReadFile(repoRelLexer)

  // runtime bits
  rtDir := filepath.Join("runtime", "c")
  rtC := filepath.Join(rtDir, "desi_std.c")
  rtH := filepath.Join(rtDir, "desi_std.h")
  rtCSrc, _ := os.ReadFile(rtC)
  rtHSrc, _ := os.ReadFile(rtH)

  // wrapper source
  wrapper := buildWrapper(entryAbs)

  // cache key
  sig := sha256Sum(
    "parsebridge-v3",
    runtime.GOOS, runtime.GOARCH,
    string(wrapper),
    string(devParserSrc),
    string(devLexerSrc),
    string(rtCSrc), string(rtHSrc),
  )
  cacheRoot := filepath.Join("gen", "tmp", "parsebridge", "cache", sig)
  binPath := filepath.Join(cacheRoot, "parsebridge_run")
  if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binPath), ".exe") {
    binPath += ".exe"
  }

  // fast path
  if fileExists(binPath) {
    return runAndClean(binPath, cacheRoot, verbose)
  }

  // work dir
  if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
    return nil, fmt.Errorf("mkdir %s: %w", cacheRoot, err)
  }

  // mirror dev parser/lexer
  if err := mirrorDevParserInto(cacheRoot, devParserSrc); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    return nil, err
  }
  if len(devLexerSrc) > 0 {
    if err := mirrorDevLexerInto(cacheRoot, devLexerSrc); err != nil {
      if !keepTmp {
        _ = os.RemoveAll(cacheRoot)
      }
      return nil, err
    }
  }

  // write wrapper
  wrapperPath := filepath.Join(cacheRoot, "main.desi")
  if err := os.WriteFile(wrapperPath, wrapper, 0o644); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    return nil, fmt.Errorf("write wrapper: %w", err)
  }

  // parse+typecheck wrapper locally
  merged, perr := resolveAndParseLocal(cacheRoot, wrapperPath)
  if len(perr) > 0 {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    var b strings.Builder
    for _, e := range perr {
      term.Bprintf(&b, "error: %v\n", e)
    }
    return nil, fmt.Errorf("resolve/parse wrapper failed:\n%s", b.String())
  }
  info, errs, _ := check.CheckFile(merged)
  if len(errs) > 0 {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    var b strings.Builder
    for _, e := range errs {
      term.Bprintf(&b, "error: %v\n", e)
    }
    return nil, fmt.Errorf("typecheck wrapper failed:\n%s", b.String())
  }

  // emit C and compile
  cpath := filepath.Join(cacheRoot, "main.c")
  csrc := cgen.EmitFile(merged, info)
  if err := os.WriteFile(cpath, []byte(csrc), 0o644); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    return nil, fmt.Errorf("write %s: %w", cpath, err)
  }
  if err := compileBin(cpath, rtDir, binPath, verbose); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    if verbose {
      return nil, fmt.Errorf("cc failed: %w", err)
    }
    return nil, fmt.Errorf("cc failed; re-run with --bridge-verbose to see compiler output")
  }

  // run bridge program and return cleaned JSON
  return runAndClean(binPath, cacheRoot, verbose)
}

/* ---------- wrapper and helpers ---------- */

func buildWrapper(entryAbs string) []byte {
  pathLit := escapeDesiString(entryAbs)
  var b strings.Builder
  b.WriteString("import compiler.desi.parser\n\n")
  b.WriteString("def main() -> int:\n")
  b.WriteString("  let path = ")
  b.WriteString(pathLit)
  b.WriteString("\n")
  b.WriteString("  let js = parse_to_json(path)\n")
  b.WriteString("  io.println(js)\n")
  b.WriteString("  return 0\n")
  b.WriteString("\n")
  return []byte(b.String())
}

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

func mirrorDevParserInto(root string, devParserSrc []byte) error {
  dstDir := filepath.Join(root, "compiler", "desi")
  if err := os.MkdirAll(dstDir, 0o755); err != nil {
    return fmt.Errorf("mkdir %s: %w", dstDir, err)
  }
  dst := filepath.Join(dstDir, "parser.desi")
  if err := os.WriteFile(dst, devParserSrc, 0o644); err != nil {
    return fmt.Errorf("write %s: %w", dst, err)
  }
  return nil
}

func mirrorDevLexerInto(root string, devLexerSrc []byte) error {
  dstDir := filepath.Join(root, "compiler", "desi")
  if err := os.MkdirAll(dstDir, 0o755); err != nil {
    return fmt.Errorf("mkdir %s: %w", dstDir, err)
  }
  dst := filepath.Join(dstDir, "lexer.desi")
  if err := os.WriteFile(dst, devLexerSrc, 0o644); err != nil {
    return fmt.Errorf("write %s: %w", dst, err)
  }
  return nil
}

// compileBin compiles C to outBin using clang.
func compileBin(cpath, rtDir, outBin string, verbose bool) error {
  clang := "clang"
  args := []string{
    cpath,
    filepath.Join(rtDir, "desi_std.c"),
    "-I", rtDir,
    "-D_CRT_SECURE_NO_WARNINGS",
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

// runAndClean executes the bridge program, dumps raw+clean outputs for debugging,
// and returns the cleaned JSON bytes.
func runAndClean(binPath, cacheRoot string, verbose bool) ([]byte, error) {
  absBin, _ := filepath.Abs(binPath)
  cmd := exec.Command(absBin)
  var out bytes.Buffer
  if verbose {
    cmd.Stderr = os.Stderr
  } else {
    cmd.Stderr = io.Discard
  }
  cmd.Stdout = &out
  if err := cmd.Run(); err != nil {
    if verbose {
      return nil, fmt.Errorf("run parsebridge: %w", err)
    }
    return nil, fmt.Errorf("run parsebridge failed; re-run with --bridge-verbose for details")
  }

  raw := out.Bytes()
  if verbose {
    _ = os.WriteFile(filepath.Join(cacheRoot, "bridge_raw.out.txt"), raw, 0o644)
  }

  clean, err := sanitizeJSONOutput(raw)
  if err != nil {
    // When verbose, also echo the raw to stderr to help eyeball it quickly.
    if verbose {
      _, _ = fmt.Fprintln(os.Stderr, "---- parsebridge raw stdout (first 4KB) ----")
      lim := raw
      if len(lim) > 4096 {
        lim = lim[:4096]
      }
      _, _ = os.Stderr.Write(lim)
      _, _ = fmt.Fprintln(os.Stderr, "\n---- end raw ----")
    }
    return nil, err
  }

  if verbose {
    _ = os.WriteFile(filepath.Join(cacheRoot, "bridge_clean.json"), clean, 0o644)
  }
  return clean, nil
}

/* ---------- small helpers (sha, fs, etc.) ---------- */

func sha256Sum(parts ...string) string {
  h := sha256.New()
  for _, p := range parts {
    _, _ = io.WriteString(h, p)
    _, _ = io.WriteString(h, "\x00")
  }
  return hex.EncodeToString(h.Sum(nil))
}

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

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
func mustAbs(p string) string  { a, _ := filepath.Abs(p); return a }
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
