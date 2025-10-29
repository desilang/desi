package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lex"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/token"
)

var (
	flagVersion    = flag.Bool("version", false, "print version and exit")
	flagDiag       = flag.Bool("diag", false, "emit a sample diagnostic and exit")
	flagDemoTokens = flag.Bool("demo-tokens", false, "print a small token/category demo and exit")
	flagDemoLayout = flag.Bool("demo-layout", false, "print layout tokens for a small sample and exit")
	flagTokens     = flag.String("tokens", "", "scan the given .desi file and print tokens")
	flagAST        = flag.String("ast", "", "parse the given .desi file and pretty-print the AST")
	flagCheck      = flag.String("check", "", "parse + resolve/check the given .desi file")
	flagIRoots     = flag.String("I", "", "colon-separated import roots (e.g., 'examples:compiler/lib')")
	flagVerbose    = flag.Bool("v", false, "verbose output")
)

var parserCodeMap = map[string]string{
	"DPE0001": "parser.unexpected_token",
	"DPE0002": "parser.expected_token",
	"DPE0003": "parser.unclosed_delimiter",
	"DPE0004": "parser.trailing_or_extra_token",
	"DPE0005": "parser.invalid_assignment_target",
	"DPE1001": "parser.async_before_def",
	"DPE1002": "parser.async_before_let",
}

const Version = "0.0.1-revised-bootstrap"

func main() {
	// If user used the subcommand form, handle it with our own permissive parser
	// so flags can be before OR after the filename.
	if len(os.Args) >= 2 && os.Args[1] == "check" {
		file, roots, verbose, err := parseCheckArgs(os.Args[2:])
		if err != nil {
			term.Eprintln("check error:", err)
			term.Flush()
			os.Exit(2)
		}

		if verbose {
			if abs, err := filepath.Abs(file); err == nil {
				term.Eprintln("check:", abs)
			} else {
				term.Eprintln("check:", file)
			}
			if roots == "" {
				term.Eprintln("roots: (none)")
			} else {
				term.Eprintln("roots:", roots)
			}
		}

		hadErrors, runErr := runCheck(file, roots)
		if runErr != nil {
			term.Eprintln("check error:", runErr)
			term.Flush()
			os.Exit(2)
		}
		if hadErrors {
			term.Flush()
			os.Exit(1)
		}
		term.Flush()
		os.Exit(0)
	}

	// Otherwise, use the global flags (supports: desic -I ROOTS -check FILE)
	flag.Parse()

	// Also support: desic -I ROOTS check FILE  (global flags parse -I; leftover args carry "check FILE")
	args := flag.Args()
	if len(args) >= 2 && args[0] == "check" && *flagCheck == "" {
		*flagCheck = args[1]
	}

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

	if *flagAST != "" {
		if err := dumpAST(*flagAST); err != nil {
			term.Eprintln("parse error:", err)
			term.Flush()
			os.Exit(2)
		}
		term.Flush()
		return
	}

	if *flagCheck != "" {
		if *flagVerbose {
			if abs, err := filepath.Abs(*flagCheck); err == nil {
				term.Eprintln("check:", abs)
			} else {
				term.Eprintln("check:", *flagCheck)
			}
			if *flagIRoots == "" {
				term.Eprintln("roots: (none)")
			} else {
				term.Eprintln("roots:", *flagIRoots)
			}
		}

		hadErrors, err := runCheck(*flagCheck, *flagIRoots)
		if err != nil {
			term.Eprintln("check error:", err)
			term.Flush()
			os.Exit(2)
		}
		if hadErrors {
			term.Flush()
			os.Exit(1)
		}
		term.Flush()
		os.Exit(0)
	}

	term.Println("desic: Try -diag, -version, -demo-tokens, -demo-layout, -tokens <file>, -ast <file>, or `check <file>`.")
	term.Flush()
}

// parseCheckArgs accepts flags in any order after `check` and returns (file, roots, verbose).
// Supports: -I ROOTS, -I=ROOTS, -v, and "--" to end flags.
func parseCheckArgs(argv []string) (string, string, bool, error) {
	var file string
	var roots string
	var verbose bool

	sawSep := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			sawSep = true
			continue
		}
		if !sawSep && strings.HasPrefix(a, "-") {
			switch {
			case a == "-v":
				verbose = true
			case a == "-I":
				if i+1 >= len(argv) {
					return "", "", false, fmt.Errorf("missing value for -I")
				}
				roots = argv[i+1]
				i++
			case strings.HasPrefix(a, "-I="):
				roots = strings.TrimPrefix(a, "-I=")
			default:
				return "", "", false, fmt.Errorf("unknown flag %q", a)
			}
			continue
		}
		// positional
		if file == "" {
			file = a
		}
	}

	if file == "" {
		return "", "", false, fmt.Errorf("missing <file> for `desic check`")
	}
	return file, roots, verbose, nil
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
	items, scanErrs, err := lex.ScanFileFull(path)
	if err != nil {
		return err
	}
	for _, it := range items {
		tag := ""
		if it.Tok == token.IDENT && it.Lexeme != "" && token.IsBuiltinType(it.Lexeme) {
			tag = " (type)"
		}
		term.Printf("%-10s %-12q%s  @%d:%d\n", it.Tok.String(), it.Lexeme, tag, it.Line, it.Col)
	}
	term.Flush()

	if len(scanErrs) > 0 {
		const maxLexErrs = 50
		p := filepath.Join("compiler", "internal", "diag", "codes.json")
		f, err := os.Open(p)
		if err == nil {
			defer func() { _ = f.Close() }()
			if cat, err := diag.LoadCatalog(f); err == nil {
				b := diag.NewBuilder(cat)
				limit := len(scanErrs)
				if limit > maxLexErrs {
					limit = maxLexErrs
				}
				for i := 0; i < limit; i++ {
					se := scanErrs[i]
					code := se.CodePath
					if code == "" {
						code = "lexer.generic_lexer_error"
					}
					d, err := b.New(code, diag.Label{
						Span: diag.Span{
							File:  se.File,
							Start: diag.Pos{Line: se.Line, Col: se.Col},
							End:   diag.Pos{Line: se.Line, Col: se.Col},
						},
						Text:    se.Message,
						Primary: true,
					})
					if err == nil {
						d.RenderTTY(os.Stderr, diag.Theme{Color: false})
					} else {
						term.Eprintln("lexer error:", se.Message)
					}
				}
				if extra := len(scanErrs) - limit; extra > 0 {
					term.Eprintln("…", extra, "more lexer errors suppressed")
				}
			}
		}
	}
	term.Flush()
	return nil
}

func dumpAST(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	root, pdiags := parse.ParseFile(path, b)

	ast.Print(os.Stdout, root)
	term.Flush()

	if len(pdiags) > 0 {
		const _max = 50
		p := filepath.Join("compiler", "internal", "diag", "codes.json")
		if f, err := os.Open(p); err == nil {
			defer func() { _ = f.Close() }()
			if cat, err := diag.LoadCatalog(f); err == nil {
				bld := diag.NewBuilder(cat)
				limit := len(pdiags)
				if limit > _max {
					limit = _max
				}
				for i := 0; i < limit; i++ {
					d := pdiags[i]
					codePath := parserCodeMap[d.CodeID]
					if codePath == "" {
						codePath = "parser.unexpected_token"
					}
					dd, err := bld.New(codePath, d.Primary, diag.WithMessage(d.Message))
					if err == nil {
						dd.RenderTTY(os.Stderr, diag.Theme{Color: false})
					} else {
						d.RenderTTY(os.Stderr, diag.Theme{Color: false})
					}
				}
				if extra := len(pdiags) - limit; extra > 0 {
					term.Eprintln("…", extra, "more parser errors suppressed")
				}
			}
		}
	}
	return nil
}

func runCheck(path, iroots string) (hadErrors bool, err error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	mod, pdiags := parse.ParseFile(path, src)
	if len(pdiags) > 0 {
		for _, d := range pdiags {
			d.RenderTTY(os.Stderr, diag.Theme{Color: false})
		}
		return true, nil
	}

	// Build loader from -I roots (colon-separated).
	var loader resolve.Loader
	roots := splitRoots(iroots)
	if len(roots) > 0 && roots[0] != "" {
		loader = resolve.NewFSLoaderMulti(roots)
	} else {
		loader = resolve.NewMemLoader(nil)
	}

	res := check.CheckWithLoader(mod, loader)
	if len(res.Diags) == 0 {
		term.Println("ok")
		return false, nil
	}
	for _, d := range res.Diags {
		d.RenderTTY(os.Stderr, diag.Theme{Color: false})
	}
	return true, nil
}

func splitRoots(s string) []string {
	if s == "" {
		return nil
	}
	// Your CLI examples use ":" (macOS/Linux); keep it simple.
	return strings.Split(s, ":")
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
		Text:    "expected int, found str",
		Primary: true,
	}

	d, err := b.New("type.type_mismatch", primary,
		diag.WithNotes("expected type int", "found type str"),
	)
	if err != nil {
		return err
	}

	d.RenderTTY(os.Stderr, diag.Theme{Color: false})
	return nil
}
