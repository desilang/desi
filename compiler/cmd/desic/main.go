package main

import (
  "bytes"
  "flag"
  "os"
  "os/exec"
  "path/filepath"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/build"
  "github.com/desilang/desi/compiler/internal/check"
  cgen "github.com/desilang/desi/compiler/internal/codegen/c"
  "github.com/desilang/desi/compiler/internal/lexer"
  "github.com/desilang/desi/compiler/internal/parser"
  "github.com/desilang/desi/compiler/internal/term"
  "github.com/desilang/desi/compiler/internal/version"
)

func usage() {
  term.Eprintln("desic — Desi compiler (Stage-1)")
  term.Eprintln("")
  term.Eprintln("Usage:")
  term.Eprintln("  desic <command> [args]")
  term.Eprintln("")
  term.Eprintln("Commands:")
  term.Eprintln("  version                          Print version")
  term.Eprintln("  help                             Show this help")
  term.Eprintln("  lex <file>                       Lex a .desi file (Go lexer) and print tokens")
  term.Eprintln("  parse <file>                     Parse a .desi file and print AST outline")
  term.Eprintln("  build [--cc=clang] [--out=name] [--Werror] <entry.desi>")
  term.Eprintln("                                    (flags may appear before or after the file)")
  term.Eprintln("  lex-desi [--keep-tmp] [--format=raw|ndjson] <file>")
  term.Eprintln("                                    EXPERIMENTAL: run Desi lexer (compiler.desi.lexer) and print tokens")
  term.Eprintln("")
  term.Eprintln("Notes:")
  term.Eprintln("  - Imports like 'foo.bar' resolve to 'foo/bar.desi' relative to the entry file’s dir.")
  term.Eprintln("  - Imports starting with 'std.' are ignored in Stage-0/1 (provided by runtime).")
  term.Eprintln("")
  term.Eprintln("Outputs:")
  term.Eprintln("  generated C:   gen/out/<basename>.c")
  term.Eprintln("  binary (if --cc): gen/out/<out|basename>")
}

/* ---------- lex (Go lexer) ---------- */

func cmdLexDirect(path string) int {
  data, err := os.ReadFile(path)
  if err != nil {
    term.Eprintf("read %s: %v\n", path, err)
    return 1
  }
  lx := lexer.New(string(data))
  for {
    t := lx.Next()
    if t.Kind == lexer.TokEOF {
      term.Printf("%d:%d  %s\n", t.Line, t.Col, t.Kind)
      break
    }
    lex := t.Lex
    if len(lex) > 40 {
      lex = lex[:37] + "..."
    }
    if lex == "" {
      term.Printf("%d:%d  %-8s\n", t.Line, t.Col, t.Kind)
    } else {
      term.Printf("%d:%d  %-8s  %q\n", t.Line, t.Col, t.Kind, lex)
    }
  }
  return 0
}

/* ---------- parse ---------- */

func cmdParse(args []string) int {
  if len(args) != 1 {
    term.Eprintln("usage: desic parse <file.desi>")
    return 2
  }
  data, err := os.ReadFile(args[0])
  if err != nil {
    term.Eprintf("read %s: %v\n", args[0], err)
    return 1
  }
  p := parser.New(string(data))
  f, perr := p.ParseFile()
  if perr != nil {
    term.Eprintf("parse: %v\n", perr)
    return 1
  }
  out := ast.DumpFile(f)
  term.Printf("%s", out)
  return 0
}

/* ---------- build (flags anywhere) ---------- */

type buildArgs struct {
  cc   string
  out  string
  file string
  werr bool // --Werror
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
    term.Eprintln("usage: desic build [--cc=clang] [--out=name] [--Werror] <entry.desi>")
    return 2
  }

  // Multi-file resolve + parse (entry + imports)
  merged, perr := build.ResolveAndParse(a.file)
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

  // Optionally compile to gen/out/<out|basename>
  if a.cc != "" {
    outName := a.out
    if outName == "" {
      outName = base
    }
    binPath := filepath.Join(outDir, outName)
    cmd := exec.Command(a.cc,
      cpath,
      filepath.Join("runtime", "c", "desi_std.c"),
      "-I", filepath.Join("runtime", "c"),
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

// tiny local helper so main.go doesn't import check directly
func cgenCheckFileShim(f *ast.File) (*check.Info, []error, []check.Warning) {
  return check.CheckFile(f)
}

/* ---------- EXPERIMENTAL: run Desi lexer (compiler.desi.lexer) ---------- */

type lexDesiArgs struct {
  file    string
  keepTmp bool
  format  string // "raw" or "ndjson"
}

func parseLexDesiArgs(argv []string) (lexDesiArgs, error) {
  var a lexDesiArgs
  a.format = "raw"
  i := 0
  for i < len(argv) {
    s := argv[i]
    switch {
    case s == "--keep-tmp":
      a.keepTmp = true
      i++
      continue
    case strings.HasPrefix(s, "--format="):
      a.format = strings.TrimPrefix(s, "--format=")
      if a.format != "raw" && a.format != "ndjson" {
        return a, flag.ErrHelp
      }
      i++
      continue
    }
    if !strings.HasPrefix(s, "-") && a.file == "" {
      a.file = s
      i++
      continue
    }
    // unknown flag or extra positional
    if strings.HasPrefix(s, "-") {
      return a, flag.ErrHelp
    }
    i++
  }
  if a.file == "" {
    return a, flag.ErrHelp
  }
  return a, nil
}

// We generate the wrapper under gen/tmp/lexbridge and also copy the working
// Desi lexer from examples/compiler/desi/lexer.desi into a temp import tree
// so that the import "compiler.desi.lexer" resolves relative to the wrapper.
func cmdLexDesiRun(args []string) int {
  a, err := parseLexDesiArgs(args)
  if err != nil {
    term.Eprintln("usage: desic lex-desi [--keep-tmp] [--format=raw|ndjson] <file.desi>")
    return 2
  }

  // Read input source
  data, rerr := os.ReadFile(a.file)
  if rerr != nil {
    term.Eprintf("read %s: %v\n", a.file, rerr)
    return 1
  }

  // Prep temp dirs
  tmpRoot := filepath.Join("gen", "tmp", "lexbridge")
  tmpWrapper := filepath.Join(tmpRoot, "main.desi")
  tmpImportDir := filepath.Join(tmpRoot, "compiler", "desi")
  tmpLexerPath := filepath.Join(tmpImportDir, "lexer.desi")

  // Always start clean
  _ = os.RemoveAll(tmpRoot)
  if merr := os.MkdirAll(tmpImportDir, 0o755); merr != nil {
    term.Eprintf("mkdir %s: %v\n", tmpImportDir, merr)
    return 1
  }

  // Copy the current dev lexer into the temp import tree
  srcLexer := filepath.Join("examples", "compiler", "desi", "lexer.desi")
  if cerr := copyFile(srcLexer, tmpLexerPath); cerr != nil {
    term.Eprintf("copy lexer: %v\n", cerr)
    return 1
  }

  // Build wrapper that calls lex_tokens on the provided source
  srcLiteral := escapeForDesiString(string(data))
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

  if werr := os.WriteFile(tmpWrapper, []byte(wrapper), 0o644); werr != nil {
    term.Eprintf("write wrapper: %v\n", werr)
    return 1
  }

  // Build the wrapper to a runnable binary (clang) using the same pipeline.
  binPath := filepath.Join("gen", "out", "lexbridge_run")
  rc := cmdBuild([]string{"--cc=clang", "--out=lexbridge_run", tmpWrapper})
  if rc != 0 {
    if !a.keepTmp {
      _ = os.RemoveAll(tmpRoot)
    }
    return rc
  }

  // Run it and capture stdout (encoded token stream).
  cmd := exec.Command(binPath)
  var out bytes.Buffer
  cmd.Stdout = &out
  cmd.Stderr = os.Stderr
  if rerr := cmd.Run(); rerr != nil {
    term.Eprintf("run wrapper: %v\n", rerr)
    if !a.keepTmp {
      _ = os.RemoveAll(tmpRoot)
    }
    return 1
  }

  // Convert/emit according to requested format.
  raw := out.String() // "KIND|TEXT|LINE|COL\n"...
  switch a.format {
  case "raw":
    // Mirror ERR tokens to stderr as LEXERR lines too.
    mirrorErrsToStderr(raw)
    term.Printf("%s", raw)
  case "ndjson":
    nd := convertRawToNDJSON(raw, true) // also mirror ERR to stderr
    term.Printf("%s", nd)
  default:
    // shouldn't happen due to parse guard
    term.Printf("%s", raw)
  }

  // Cleanup temp files unless asked to keep them
  if !a.keepTmp {
    _ = os.RemoveAll(tmpRoot)
  }
  return 0
}

// Escape a raw source string as a Desi string literal.
// We rely on C string escapes since Stage-1 strings are lowered directly.
func escapeForDesiString(s string) string {
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
        // control char → emit as octal \ooo
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

// copyFile reads from src and writes to dst, creating parent dirs already ensured by caller.
func copyFile(src, dst string) error {
  data, err := os.ReadFile(src)
  if err != nil {
    return err
  }
  return os.WriteFile(dst, data, 0o644)
}

/* ---------- helpers: token formatting ---------- */

// mirrorErrsToStderr scans raw "K|T|L|C" lines and writes LEXERR lines to stderr for ERR tokens.
func mirrorErrsToStderr(raw string) {
  lines := strings.Split(raw, "\n")
  for _, ln := range lines {
    if ln == "" {
      continue
    }
    parts := strings.SplitN(ln, "|", 4)
    if len(parts) != 4 {
      continue
    }
    kind, text, line, col := parts[0], parts[1], parts[2], parts[3]
    if kind == "ERR" {
      // Escape quotes in message
      msg := strings.ReplaceAll(text, `"`, `\"`)
      term.Eprintf("LEXERR line=%s col=%s msg=\"%s\"\n", line, col, msg)
    }
  }
}

// convertRawToNDJSON transforms "K|T|L|C" lines into NDJSON.
// If mirrorErr is true, it also emits LEXERR lines for ERR tokens.
func convertRawToNDJSON(raw string, mirrorErr bool) string {
  var b strings.Builder
  lines := strings.Split(raw, "\n")
  for _, ln := range lines {
    if ln == "" {
      continue
    }
    parts := strings.SplitN(ln, "|", 4)
    if len(parts) != 4 {
      // skip malformed line
      continue
    }
    kind, text, line, col := parts[0], parts[1], parts[2], parts[3]
    if mirrorErr && kind == "ERR" {
      msg := strings.ReplaceAll(text, `"`, `\"`)
      term.Eprintf("LEXERR line=%s col=%s msg=\"%s\"\n", line, col, msg)
    }
    // JSON-escape text field minimally
    esc := strings.ReplaceAll(text, `\`, `\\`)
    esc = strings.ReplaceAll(esc, `"`, `\"`)
    b.WriteString(`{"kind":"`)
    b.WriteString(kind)
    b.WriteString(`","text":"`)
    b.WriteString(esc)
    b.WriteString(`","line":`)
    b.WriteString(line)
    b.WriteString(`,"col":`)
    b.WriteString(col)
    b.WriteString("}\n")
  }
  return b.String()
}

/* ---------- main ---------- */

func main() {
  flag.Usage = usage
  if len(os.Args) < 2 {
    usage()
    return
  }
  switch os.Args[1] {
  case "version", "--version", "-v":
    term.Printf("%s\n", version.String())
  case "help", "--help", "-h":
    usage()
  case "lex":
    if len(os.Args) != 3 {
      term.Eprintln("usage: desic lex <file.desi>")
      os.Exit(2)
    }
    os.Exit(cmdLexDirect(os.Args[2]))
  case "parse":
    os.Exit(cmdParse(os.Args[2:]))
  case "build":
    os.Exit(cmdBuild(os.Args[2:]))
  case "lex-desi":
    if len(os.Args) < 3 {
      term.Eprintln("usage: desic lex-desi [--keep-tmp] [--format=raw|ndjson] <file.desi>")
      os.Exit(2)
    }
    os.Exit(cmdLexDesiRun(os.Args[2:]))
  default:
    term.Eprintf("unknown command: %s\n\n", os.Args[1])
    usage()
    os.Exit(2)
  }
}
