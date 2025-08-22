package main

import (
  "flag"
  "os"
  "os/exec"
  "path/filepath"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  cgen "github.com/desilang/desi/compiler/internal/codegen/c"
  "github.com/desilang/desi/compiler/internal/lexer"
  "github.com/desilang/desi/compiler/internal/parser"
  "github.com/desilang/desi/compiler/internal/term"
  "github.com/desilang/desi/compiler/internal/version"
)

func usage() {
  term.Eprintln("desic — Desi compiler (Stage-0)")
  term.Eprintln("")
  term.Eprintln("Usage:")
  term.Eprintln("  desic <command> [args]")
  term.Eprintln("")
  term.Eprintln("Commands:")
  term.Eprintln("  version                    Print version")
  term.Eprintln("  help                       Show this help")
  term.Eprintln("  lex <file>                 Lex a .desi file and print tokens")
  term.Eprintln("  parse <file>               Parse a .desi file and print AST outline")
  term.Eprintln("  build [--cc=clang] [--out=name] <file.desi>")
  term.Eprintln("        (flags may appear before or after the file)")
  term.Eprintln("")
  term.Eprintln("Outputs:")
  term.Eprintln("  generated C:  gen/out/<basename>.c")
  term.Eprintln("  binary (if --cc): gen/out/<out|basename>")
}

/* ---------- lex ---------- */

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
  f, err := p.ParseFile()
  if err != nil {
    term.Eprintf("parse: %v\n", err)
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
    if strings.HasPrefix(s, "--cc=") {
      a.cc = s[len("--cc="):]
      i++
      continue
    }
    if s == "--cc" {
      if i+1 >= len(argv) {
        return a, flag.ErrHelp
      }
      a.cc = argv[i+1]
      i += 2
      continue
    }
    if strings.HasPrefix(s, "--out=") {
      a.out = s[len("--out="):]
      i++
      continue
    }
    if s == "--out" {
      if i+1 >= len(argv) {
        return a, flag.ErrHelp
      }
      a.out = argv[i+1]
      i += 2
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
    term.Eprintln("usage: desic build [--cc=clang] [--out=name] <file.desi>")
    return 2
  }
  data, err := os.ReadFile(a.file)
  if err != nil {
    term.Eprintf("read %s: %v\n", a.file, err)
    return 1
  }
  p := parser.New(string(data))
  f, err := p.ParseFile()
  if err != nil {
    term.Eprintf("parse: %v\n", err)
    return 1
  }

  // Emit C to gen/out
  base := strings.TrimSuffix(filepath.Base(a.file), filepath.Ext(a.file))
  outDir := filepath.Join("gen", "out")
  if err := os.MkdirAll(outDir, 0o755); err != nil {
    term.Eprintf("mkdir %s: %v\n", outDir, err)
    return 1
  }
  cpath := filepath.Join(outDir, base+".c")
  csrc := cgen.EmitFile(f)
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
    cmd := exec.Command(a.cc, cpath, "-o", binPath)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
      term.Eprintf("cc failed: %v\n", err)
      return 1
    }
    term.Eprintf("built %s\n", binPath)
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
  default:
    term.Eprintf("unknown command: %s\n\n", os.Args[1])
    usage()
    os.Exit(2)
  }
}
