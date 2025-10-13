package main

import (
  "encoding/json"
  "flag"
  "os"
  "path/filepath"

  "github.com/desilang/desi/compiler/internal/diag"
  "github.com/desilang/desi/compiler/internal/term"
)

var (
  flagVersion = flag.Bool("version", false, "print version and exit")
  flagDiag    = flag.Bool("diag", false, "emit a sample diagnostic and exit")
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

  // TODO: add subcommands: build, check, parse, tokens, etc.
  term.Println("desic: TODO (rev6 bootstrap). Try -diag or -version.")
  term.Flush()
}

func demoDiag() error {
  // Load docs/spec/codes.json
  p := filepath.Join("docs", "spec", "codes.json")
  f, err := os.Open(p)
  if err != nil {
    return err
  }
  defer func() { _ = f.Close() }() // explicitly ignore close error

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

  d.RenderTTY(os.Stderr, diag.Theme{Color: false})

  // Also render JSON (for IDEs)
  enc := json.NewEncoder(os.Stdout)
  enc.SetIndent("", "  ")
  return enc.Encode(d)
}
