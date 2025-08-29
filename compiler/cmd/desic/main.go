package main

import (
  "flag"
  "os"
  "os/exec"
  "path/filepath"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/build"
  "github.com/desilang/desi/compiler/internal/check"
  cgen "github.com/desilang/desi/compiler/internal/codegen/c"
  "github.com/desilang/desi/compiler/internal/lexbridge"
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
  term.Eprintln("  lex-desi [--keep-tmp] [--format=raw|ndjson|pretty] <file>")
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
  format  string // "raw" | "ndjson" | "pretty"
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
      if a.format != "raw" && a.format != "ndjson" && a.format != "pretty" {
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

func cmdLexDesiRun(args []string) int {
  a, err := parseLexDesiArgs(args)
  if err != nil {
    term.Eprintln("usage: desic lex-desi [--keep-tmp] [--format=raw|ndjson|pretty] <file.desi>")
    return 2
  }

  raw, rerr := lexbridge.BuildAndRunRaw(a.file, a.keepTmp)
  if rerr != nil {
    term.Eprintf("lex-desi: %v\n", rerr)
    return 1
  }

  switch a.format {
  case "raw":
    lexbridge.MirrorErrsToStderr(raw)
    term.Printf("%s", raw)
  case "ndjson":
    nd := lexbridge.ConvertRawToNDJSON(raw, true)
    term.Printf("%s", nd)
  case "pretty":
    nd := lexbridge.ConvertRawToNDJSON(raw, true)
    toks, perr := lexbridge.ParseNDJSON(strings.NewReader(nd))
    if perr != nil {
      term.Eprintf("ndjson parse warning: %v\n", perr)
    }
    pretty := lexbridge.DebugFormat(toks, 0)
    term.Printf("%s", pretty)
  default:
    term.Printf("%s", raw)
  }
  return 0
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
      term.Eprintln("usage: desic lex-desi [--keep-tmp] [--format=raw|ndjson|pretty] <file.desi>")
      os.Exit(2)
    }
    os.Exit(cmdLexDesiRun(os.Args[2:]))
  default:
    term.Eprintf("unknown command: %s\n\n", os.Args[1])
    usage()
    os.Exit(2)
  }
}
