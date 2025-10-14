package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lex"
	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/token"
)

var (
	flagVersion    = flag.Bool("version", false, "print version and exit")
	flagDiag       = flag.Bool("diag", false, "emit a sample diagnostic and exit")
	flagDemoTokens = flag.Bool("demo-tokens", false, "print a small token/category demo and exit")
	flagDemoLayout = flag.Bool("demo-layout", false, "print layout tokens for a small sample and exit")
	flagTokens     = flag.String("tokens", "", "scan the given .desi file and print tokens")
)

const Version = "0.0.1-revised-bootstrap"

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

	if *flagDemoLayout {
		demoLayout()
		term.Flush()
		return
	}

	if *flagTokens != "" {
		if err := dumpTokens(*flagTokens); err != nil {
			term.Eprintln("scan error:", err)
			term.Flush()
			os.Exit(2)
		}
		term.Flush()
		return
	}

	// TODO: add subcommands: build, check, parse, tokens, etc.
	term.Println("desic: TODO (revised bootstrap). Try -diag, -version, -demo-tokens, -demo-layout, or -tokens <file>.")
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

func demoLayout() {
	sample := `
def hello:
  if cond:
    return 1
  else:
    return 2

def world:
  return 0
`
	toks := lex.Layoutize([]byte(sample))
	term.Println("layout events for sample:")
	for _, t := range toks {
		term.Printf("  %s\n", t.String())
	}
}

func dumpTokens(path string) error {
	items, err := lex.ScanFile(path)
	if err != nil {
		return err
	}
	for _, it := range items {
		// Annotate builtin types even though we keep them as IDENT tokens.
		tag := ""
		if it.Tok == token.IDENT && it.Lexeme != "" && token.IsBuiltinType(it.Lexeme) {
			tag = " (type)"
		}
		term.Printf("%-10s %-12q%s  @%d:%d\n", it.Tok.String(), it.Lexeme, tag, it.Line, it.Col)
	}
	return nil
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

	d.RenderTTY(os.Stderr, diag.Theme{Color: false})
	return nil
}
