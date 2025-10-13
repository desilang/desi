package main

import (
  "flag"
  "os"
  "path/filepath"

  "github.com/desilang/desi/compiler/internal/diag"
  "github.com/desilang/desi/compiler/internal/term"
  "github.com/desilang/desi/compiler/internal/token"
)

var (
  flagVersion    = flag.Bool("version", false, "print version and exit")
  flagDiag       = flag.Bool("diag", false, "emit a sample diagnostic and exit")
  flagDemoTokens = flag.Bool("demo-tokens", false, "print a small token/category demo and exit")
)

const Version = "0.0.1-rev6-bootstrap"

func main() {
  flag.Parse()
  if *flagVersion {
    term.Println("desic", Version)
    term.Flush()
    return
  }

  if *flagDiag {
    if err := demoDiag(); err != nil {
      term.Eprintln("diag error:", err)
      term.Flush()
      os.Exit(2)
    }
    term.Flush()
    return
  }

  if *flagDemoTokens {
    demoTokens()
    term.Flush()
    return
  }

  // TODO: add subcommands: build, check, parse, tokens, etc.
  term.Println("desic: TODO (rev6 bootstrap). Try -diag, -version, or -demo-tokens.")
  term.Flush()
}

func demoTokens() {
  samples := []token.Token{
    token.KW_def, token.KW_async, token.ARROW, token.FAT_ARROW,
    token.PIPE_GT, token.KW_for, token.KW_in, token.COMMA,
    token.STR, token.IDENT, token.NL,
  }
  for _, t := range samples {
    term.Printf("%-10s  cat=%v\n", t.String(), token.TokenCategory(t))
  }
}

func demoDiag() error {
  p := filepath.Join("compiler", "internal", "diag", "codes.json")
  f, err := os.Open(p)
  if err != nil {
    return err
  }
  defer func() { _ = f.Close() }()

  cat, err := diag.LoadCatalog(f)
  if err != nil {
    return err
  }
  b := diag.NewBuilder(cat)

  primary := diag.Label{
    Span: diag.Span{
      File:  "src/main.desi",
      Start: diag.Pos{Line: 4, Col: 17, Byte: 0},
      End:   diag.Pos{Line: 4, Col: 24, Byte: 0},
    },
    Text:    "expected `int`, found `str`",
    Primary: true,
  }

  d, err := b.New("type.type_mismatch", primary,
    diag.WithNotes("expected type `int`", "found type `str`"),
  )
  if err != nil {
    return err
  }

  // Print only the human-friendly form (no JSON payload).
  d.RenderTTY(os.Stderr, diag.Theme{Color: false})
  return nil
}
