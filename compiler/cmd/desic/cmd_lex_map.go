package main

import (
  "strings"

  "github.com/desilang/desi/compiler/internal/lexbridge"
  "github.com/desilang/desi/compiler/internal/term"
)

/* ---------- lex-map (report mapping coverage) ---------- */

func cmdLexMap(args []string) int {
  // args: [--verbose] <file>
  var verbose bool
  var file string
  for i := 0; i < len(args); i++ {
    s := args[i]
    if s == "--verbose" {
      verbose = true
      continue
    }
    if !strings.HasPrefix(s, "-") && file == "" {
      file = s
      continue
    }
    if strings.HasPrefix(s, "-") {
      term.Eprintln("usage: desic lex-map [--verbose] <file.desi>")
      return 2
    }
  }
  if file == "" {
    term.Eprintln("usage: desic lex-map [--verbose] <file.desi>")
    return 2
  }

  // Run Desi lexer and get NDJSON (no stale fallback)
  raw, rerr := lexbridge.BuildAndRunRaw(file, false, verbose)
  if rerr != nil {
    term.Eprintf("lex-map bridge: %v\n", rerr)
    return 1
  }
  nd := lexbridge.ConvertRawToNDJSON(raw, true)
  toks, perr := lexbridge.ParseNDJSON(strings.NewReader(nd))
  if perr != nil {
    term.Eprintf("ndjson parse warning: %v\n", perr)
  }

  // Tally coverage
  km := lexbridge.DefaultKindMap()
  cov := lexbridge.NewCoverage()
  for _, t := range toks {
    _, ok := km.MapKind(t.Kind, t.Text)
    cov.Tally(t.Kind, t.Text, ok)
  }

  term.Printf("file: %s\n", file)
  term.Printf("%s", cov.RenderReport())
  return 0
}
