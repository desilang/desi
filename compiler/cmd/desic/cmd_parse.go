package main

import (
  "fmt"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/build"
  "github.com/desilang/desi/compiler/internal/lexbridge"
  "github.com/desilang/desi/compiler/internal/term"
)

/* ---------- parse ---------- */

func cmdParse(args []string) int {
  // Accept:
  //   desic parse [--use-desi-lexer] [--keep-bridge-tmp] [--bridge-verbose] [--dump-go] <file.desi>
  useDesi := false
  keepTmp := false
  verbose := false
  dumpGo := false
  var file string

  usage := func() int {
    term.Eprintln("usage: desic parse [--use-desi-lexer] [--keep-bridge-tmp] [--bridge-verbose] [--dump-go] <file.desi>")
    return 2
  }

  for _, s := range args {
    switch {
    case s == "--use-desi-lexer":
      useDesi = true
    case s == "--keep-bridge-tmp":
      keepTmp = true
    case s == "--bridge-verbose":
      verbose = true
    case s == "--dump-go":
      dumpGo = true
    case !strings.HasPrefix(s, "-") && file == "":
      file = s
    case strings.HasPrefix(s, "-"):
      return usage()
    }
  }
  if file == "" {
    return usage()
  }

  // Unified path: choose Go-lexer or Desi-lexer bridge via loader.go helper.
  f, errs := build.ResolveAndParseMaybeDesi(file, useDesi, keepTmp, verbose)
  if len(errs) > 0 {
    for _, e := range errs {
      // Pretty-print lexbridge LEXERR diagnostics in Rust style, else fallback.
      if pretty := lexbridge.RenderLexbridgeErrorPretty(e, file, nil); pretty != "" {
        term.Eprintf("%s", pretty)
      } else {
        term.Eprintf("%v\n", e)
      }
    }
    return 1
  }

  // Debug mode: raw Go struct (shows Span{Start{Line,Col} End{...}} everywhere).
  if dumpGo {
    fmt.Printf("%#v\n", f)
    return 0
  }

  // Default: pretty AST dump (existing behavior).
  out := ast.DumpFile(f)
  term.Printf("%s", out)
  return 0
}
