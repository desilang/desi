package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/format"
	"github.com/desilang/desi/compiler/internal/lex"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/token"
	"github.com/desilang/desi/compiler/internal/version"
	"github.com/desilang/desi/compiler/lib"
)

var (
	flagVersion     = flag.Bool("version", false, "print version and exit")
	flagDiag        = flag.Bool("diag", false, "emit a sample diagnostic and exit")
	flagDemoTokens  = flag.Bool("demo-tokens", false, "print a small token/category demo and exit")
	flagDemoLayout  = flag.Bool("demo-layout", false, "print layout tokens for a small sample and exit")
	flagTokens      = flag.String("tokens", "", "scan the given .desi file and print tokens")
	flagAST         = flag.String("ast", "", "parse the given .desi file and pretty-print the AST")
	flagCheck       = flag.String("check", "", "parse + resolve/check the given .desi file")
	flagIRoots      = flag.String("I", "", "colon-separated import roots (e.g., 'examples:compiler/lib')")
	flagVerbose     = flag.Bool("v", false, "verbose output")
	flagErrorFormat = flag.String("error-format", "human", "error format: human|json")
	flagColor       = flag.String("color", "auto", "color: auto|always|never")
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

func main() {
	// --- Pre-scan render flags so 'check' subcommand also honors them ---
	ef := "human"
	colStr := "auto"
	if len(os.Args) > 1 {
		args := os.Args[1:]
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case strings.HasPrefix(a, "--error-format="):
				ef = strings.ToLower(strings.TrimPrefix(a, "--error-format="))
			case a == "--error-format" && i+1 < len(args):
				i++
				ef = strings.ToLower(args[i])
			case strings.HasPrefix(a, "--color="):
				colStr = strings.ToLower(strings.TrimPrefix(a, "--color="))
			case a == "--color" && i+1 < len(args):
				i++
				colStr = strings.ToLower(args[i])
			}
		}
	}
	var cm diag.ColorMode
	switch colStr {
	case "always":
		cm = diag.Always
	case "never":
		cm = diag.Never
	default:
		cm = diag.Auto
	}
	diag.SetGlobalRender(ef, cm)

	exitCode := 0

	// Subcommand path: desic fmt ...
	if len(os.Args) >= 2 && os.Args[1] == "fmt" {
		exitCode := runFmt(os.Args[2:])
		os.Exit(exitCode)
	}

	// Subcommand path: desic doc ...
	if len(os.Args) >= 2 && os.Args[1] == "doc" {
		exitCode := runDoc(os.Args[2:])
		os.Exit(exitCode)
	}

	// Subcommand path: desic watch ...
	if len(os.Args) >= 2 && os.Args[1] == "watch" {
		exitCode := runWatch(os.Args[2:])
		os.Exit(exitCode)
	}

	// Subcommand path: desic check ...
	if len(os.Args) >= 2 && os.Args[1] == "check" {
		jsonMode := ef == "json"
		if jsonMode {
			diag.BeginJSONCapture()
		}

		file, roots, verbose, err := parseCheckArgs(stripRenderFlags(os.Args[2:]))
		if err != nil {
			term.Eprintln("check error:", err)
			term.Flush()
			if jsonMode {
				if flushErr := diag.EndJSONCapture(os.Stderr); flushErr != nil {
					term.Eprintln("desic: json flush:", flushErr)
				}
			}
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
			exitCode = 2
		} else if hadErrors {
			exitCode = 1
		} else {
			exitCode = 0
		}

		term.Flush()
		if jsonMode {
			if flushErr := diag.EndJSONCapture(os.Stderr); flushErr != nil {
				term.Eprintln("desic: json flush:", flushErr)
			}
		}
		os.Exit(exitCode)
	}

	// Global flags path
	flag.Parse()

	// Also support: desic -I ROOTS check FILE
	args := flag.Args()
	if len(args) >= 2 && args[0] == "check" && *flagCheck == "" {
		*flagCheck = args[1]
	}

	// Re-apply render prefs from parsed flags
	var cm2 diag.ColorMode
	switch strings.ToLower(*flagColor) {
	case "always":
		cm2 = diag.Always
	case "never":
		cm2 = diag.Never
	default:
		cm2 = diag.Auto
	}
	ef = strings.ToLower(*flagErrorFormat)
	diag.SetGlobalRender(ef, cm2)
	jsonMode := ef == "json"
	if jsonMode {
		diag.BeginJSONCapture()
	}

	// Commands
	if *flagVersion {
		term.Println("desic", version.Version)
		exitCode = 0
		goto END
	}

	if *flagDiag {
		if err := demoDiag(); err != nil {
			term.Eprintln("diag error:", err)
			exitCode = 2
			goto END
		}
		exitCode = 0
		goto END
	}

	if *flagDemoTokens {
		demoTokens()
		exitCode = 0
		goto END
	}

	if *flagDemoLayout {
		demoLayout()
		exitCode = 0
		goto END
	}

	if *flagTokens != "" {
		if err := dumpTokens(*flagTokens); err != nil {
			term.Eprintln("scan error:", err)
			exitCode = 2
			goto END
		}
		exitCode = 0
		goto END
	}

	if *flagAST != "" {
		if err := dumpAST(*flagAST); err != nil {
			term.Eprintln("parse error:", err)
			exitCode = 2
			goto END
		}
		exitCode = 0
		goto END
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
			exitCode = 2
			goto END
		}
		if hadErrors {
			exitCode = 1
		} else {
			exitCode = 0
		}
		goto END
	}

	printUsage()
	exitCode = 0

END:
	term.Flush()
	if jsonMode {
		if err := diag.EndJSONCapture(os.Stderr); err != nil {
			term.Eprintln("desic: json flush:", err)
		}
	}
	os.Exit(exitCode)
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
	const maxCheckDiags = 15

	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	mod, pdiags := parse.ParseFile(path, src)

	// If parsing produced diagnostics, show up to maxCheckDiags and suppress the rest.
	if len(pdiags) > 0 {
		limit := len(pdiags)
		if limit > maxCheckDiags {
			limit = maxCheckDiags
		}
		for i := 0; i < limit; i++ {
			pdiags[i].RenderTTY(os.Stderr, diag.Theme{Color: false})
		}
		if extra := len(pdiags) - limit; extra > 0 {
			term.Eprintln("…", extra, "more errors suppressed")
		}
		return true, nil
	}

	// Build loader from -I roots + embedded stdlib + file's directory.
	var loader resolve.Loader
	roots := splitRoots(iroots)
	// Include the file's parent directory for relative imports (e.g., from bar import greet)
	fileDir := filepath.Dir(path)
	if fileDir == "" || fileDir == "." {
		fileDir, _ = os.Getwd()
	}
	roots = append(roots, fileDir)
	// Use embedded stdlib from the binary
	embedStdlib := resolve.NewEmbedFSLoader(lib.StdlibFS)
	loader = resolve.NewFSLoaderMultiWithStdlib(roots, embedStdlib)

	res := check.CheckWithLoader(mod, loader)
	if len(res.Diags) == 0 {
		term.Println("ok")
		return false, nil
	}

	// Show up to maxCheckDiags checker diagnostics; suppress the rest.
	limit := len(res.Diags)
	if limit > maxCheckDiags {
		limit = maxCheckDiags
	}
	for i := 0; i < limit; i++ {
		res.Diags[i].RenderTTY(os.Stderr, diag.Theme{Color: false})
	}
	if extra := len(res.Diags) - limit; extra > 0 {
		term.Eprintln("…", extra, "more errors suppressed")
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

// stripRenderFlags removes --error-format[=v] and --color[=v] (and their
// space-separated forms) from args. It lets `desic check --error-format=json ...` work.
func stripRenderFlags(args []string) []string {
	out := make([]string, 0, len(args))
	skipNext := false
	for i := 0; i < len(args); i++ {
		if skipNext {
			skipNext = false
			continue
		}
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--error-format="),
			a == "--error-format",
			strings.HasPrefix(a, "--color="),
			a == "--color":
			if a == "--error-format" || a == "--color" {
				// consume the next token as its value if present
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					skipNext = true
				}
			}
			// swallow this flag
			continue
		default:
			out = append(out, a)
		}
	}
	return out
}

// ---- fmt subcommand ----

// runFmt handles the `desic fmt` subcommand.
// Flags: -w (write in place), -l (list files that differ), -q (quiet)
func runFmt(args []string) int {
	writeInPlace := false
	listOnly := false
	quiet := false
	var files []string

	for _, a := range args {
		switch a {
		case "-w":
			writeInPlace = true
		case "-l":
			listOnly = true
		case "-q":
			quiet = true
		default:
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		term.Println("usage: desic fmt [flags] <file-or-dir> ...")
		term.Println("flags: -w (write in place), -l (list files that differ), -q (quiet)")
		return 2
	}

	hadError := false
	for _, path := range files {
		stat, err := os.Stat(path)
		if err != nil {
			term.Eprintln("desic fmt:", err)
			hadError = true
			continue
		}

		if stat.IsDir() {
			err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || filepath.Ext(p) != ".desi" {
					return nil
				}
				if !formatOneFile(p, writeInPlace, listOnly, quiet) {
					hadError = true
				}
				return nil
			})
			if err != nil {
				term.Eprintln("desic fmt:", err)
				hadError = true
			}
		} else {
			if !formatOneFile(path, writeInPlace, listOnly, quiet) {
				hadError = true
			}
		}
	}

	if hadError {
		return 1
	}
	return 0
}

// formatOneFile formats a single .desi file.
func formatOneFile(path string, writeInPlace, listOnly, quiet bool) bool {
	f, err := os.Open(path)
	if err != nil {
		term.Eprintln("desic fmt:", err)
		return false
	}
	defer func() { _ = f.Close() }()

	src, err := io.ReadAll(f)
	if err != nil {
		term.Eprintln("desic fmt:", err)
		return false
	}

	out, diags := format.FormatBytes(src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		return false
	}

	if listOnly {
		if !bytesEqual(src, out) {
			term.Println(path)
		}
		return true
	}

	if writeInPlace {
		if bytesEqual(src, out) {
			if !quiet {
				term.Println("formatted:", path, "(no changes)")
			}
			return true
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			term.Eprintln("desic fmt:", err)
			return false
		}
		if !quiet {
			term.Println("wrote:", path)
		}
		return true
	}

	// Default: print to stdout
	term.Write(os.Stdout, out)
	return true
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- doc subcommand ----

// runDoc handles the `desic doc` subcommand.
// Generates markdown documentation from Desi source files.
// Flags: --all (include non-pub declarations)
func runDoc(args []string) int {
	showAll := false
	var files []string

	for _, a := range args {
		switch a {
		case "--all", "-a":
			showAll = true
		default:
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		term.Println("usage: desic doc [--all] <file.desi>")
		term.Println("flags: --all, -a (include non-pub declarations)")
		term.Flush()
		return 2
	}

	path := files[0]
	src, err := os.ReadFile(path)
	if err != nil {
		term.Eprintln("desic doc:", err)
		term.Flush()
		return 1
	}

	mod, diags := parse.ParseFile(path, src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		term.Flush()
		return 1
	}

	// Generate markdown documentation
	doc := generateDoc(mod, src, showAll)
	term.Println(doc)
	term.Flush()
	return 0
}

// generateDoc generates markdown documentation from a parsed module.
// If showAll is true, includes non-pub declarations.
func generateDoc(mod *ast.Module, src []byte, showAll bool) string {
	var sb strings.Builder

	// Module header
	baseName := filepath.Base(mod.File)
	sb.WriteString(fmt.Sprintf("# Module: %s\n\n", baseName))

	// Module docstring (if present)
	if mod.Doc != nil {
		docText := extractDocString(mod.Doc, src)
		if docText != "" {
			sb.WriteString(docText + "\n\n")
		}
	}

	// Collect declarations by type
	var funcs []*ast.FuncDecl
	var classes []*ast.ClassDecl
	var structs []*ast.StructDecl
	var enums []*ast.EnumDecl

	for _, d := range mod.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			if decl.Name.Name != "__top__" && (showAll || decl.Pub) {
				funcs = append(funcs, decl)
			}
		case *ast.ClassDecl:
			if showAll || decl.Pub {
				classes = append(classes, decl)
			}
		case *ast.StructDecl:
			if showAll || decl.Pub {
				structs = append(structs, decl)
			}
		case *ast.EnumDecl:
			if showAll || decl.Pub {
				enums = append(enums, decl)
			}
		}
	}

	// Document functions
	if len(funcs) > 0 {
		sb.WriteString("## Functions\n\n")
		for _, fn := range funcs {
			renderFunc(&sb, fn, src)
		}
	}

	// Document classes
	if len(classes) > 0 {
		sb.WriteString("## Classes\n\n")
		for _, cls := range classes {
			renderClass(&sb, cls, src)
		}
	}

	// Document structs
	if len(structs) > 0 {
		sb.WriteString("## Structs\n\n")
		for _, st := range structs {
			renderStruct(&sb, st, src)
		}
	}

	// Document enums
	if len(enums) > 0 {
		sb.WriteString("## Enums\n\n")
		for _, en := range enums {
			renderEnum(&sb, en, src)
		}
	}

	return sb.String()
}

// renderFunc renders documentation for a function.
func renderFunc(sb *strings.Builder, fn *ast.FuncDecl, src []byte) {
	// Line number
	line := fn.Span.Start.Line

	// Build signature with type params
	sig := formatFuncSigFull(fn)

	// Visibility and async markers
	var markers []string
	if !fn.Pub {
		markers = append(markers, "private")
	}
	if fn.Async {
		markers = append(markers, "async")
	}
	markerStr := ""
	if len(markers) > 0 {
		markerStr = fmt.Sprintf(" *(%s)*", strings.Join(markers, ", "))
	}

	sb.WriteString(fmt.Sprintf("### `%s`%s\n", sig, markerStr))
	sb.WriteString(fmt.Sprintf("*Line %d*\n\n", line))

	// Decorators
	if len(fn.Decorators) > 0 {
		for _, dec := range fn.Decorators {
			sb.WriteString(fmt.Sprintf("- `@%s`\n", dec.Name.Name))
		}
		sb.WriteString("\n")
	}

	// Docstring
	if fn.Doc != nil {
		docText := extractDocString(fn.Doc, src)
		if docText != "" {
			sb.WriteString(docText + "\n\n")
		}
	}
}

// renderClass renders documentation for a class.
func renderClass(sb *strings.Builder, cls *ast.ClassDecl, src []byte) {
	// Line number
	line := cls.Span.Start.Line

	// Class name with type params
	className := cls.Name.Name
	if len(cls.TypeParams) > 0 {
		var tps []string
		for _, tp := range cls.TypeParams {
			tpStr := tp.Name.Name
			if len(tp.Bounds) > 0 {
				var bounds []string
				for _, b := range tp.Bounds {
					bounds = append(bounds, b.Name)
				}
				tpStr += ": " + strings.Join(bounds, " + ")
			}
			tps = append(tps, tpStr)
		}
		className += "<" + strings.Join(tps, ", ") + ">"
	}

	// Visibility marker
	visMarker := ""
	if !cls.Pub {
		visMarker = " *(private)*"
	}

	sb.WriteString(fmt.Sprintf("### `%s`%s\n", className, visMarker))
	sb.WriteString(fmt.Sprintf("*Line %d*\n\n", line))

	// Decorators
	if len(cls.Decorators) > 0 {
		for _, dec := range cls.Decorators {
			sb.WriteString(fmt.Sprintf("- `@%s`\n", dec.Name.Name))
		}
		sb.WriteString("\n")
	}

	// Inheritance
	if len(cls.Bases) > 0 {
		var bases []string
		for _, b := range cls.Bases {
			bases = append(bases, formatTypeName(b))
		}
		sb.WriteString(fmt.Sprintf("**Extends:** %s\n\n", strings.Join(bases, ", ")))
	}

	// Docstring
	if cls.Doc != nil {
		docText := extractDocString(cls.Doc, src)
		if docText != "" {
			sb.WriteString(docText + "\n\n")
		}
	}

	// Constants
	if len(cls.Constants) > 0 {
		sb.WriteString("**Constants:**\n")
		for _, c := range cls.Constants {
			vis := ""
			if !c.IsPub {
				vis = " *(private)*"
			}
			typeName := formatTypeName(c.Type)
			sb.WriteString(fmt.Sprintf("- `%s: %s`%s\n", c.Name.Name, typeName, vis))
		}
		sb.WriteString("\n")
	}

	// Static fields
	if len(cls.StaticFields) > 0 {
		sb.WriteString("**Static Fields:**\n")
		for _, sf := range cls.StaticFields {
			vis := ""
			if !sf.IsPub {
				vis = " *(private)*"
			}
			mut := ""
			if sf.IsMut {
				mut = " *(mut)*"
			}
			typeName := formatTypeName(sf.Type)
			sb.WriteString(fmt.Sprintf("- `%s: %s`%s%s\n", sf.Name.Name, typeName, mut, vis))
		}
		sb.WriteString("\n")
	}

	// Instance fields
	if len(cls.Fields) > 0 {
		sb.WriteString("**Fields:**\n")
		for _, f := range cls.Fields {
			vis := ""
			if !f.Pub {
				vis = " *(private)*"
			}
			mut := ""
			if f.Mut {
				mut = " *(mut)*"
			}
			typeName := "any"
			if f.Type != nil {
				typeName = formatTypeName(f.Type)
			}
			sb.WriteString(fmt.Sprintf("- `%s: %s`%s%s\n", f.Name.Name, typeName, mut, vis))
		}
		sb.WriteString("\n")
	}

	// Methods
	if len(cls.Methods) > 0 {
		sb.WriteString("**Methods:**\n")
		for _, m := range cls.Methods {
			sig := formatFuncSig(m)
			vis := ""
			if !m.Pub {
				vis = " *(private)*"
			}
			async := ""
			if m.Async {
				async = " *(async)*"
			}
			sb.WriteString(fmt.Sprintf("- `%s`%s%s", sig, async, vis))
			if m.Doc != nil {
				docText := extractDocString(m.Doc, src)
				if docText != "" {
					lines := strings.SplitN(docText, "\n", 2)
					sb.WriteString(fmt.Sprintf(" - %s", strings.TrimSpace(lines[0])))
				}
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
}

// renderStruct renders documentation for a struct.
func renderStruct(sb *strings.Builder, st *ast.StructDecl, src []byte) {
	// Line number
	line := st.Span.Start.Line

	// Visibility marker
	visMarker := ""
	if !st.Pub {
		visMarker = " *(private)*"
	}

	sb.WriteString(fmt.Sprintf("### `%s`%s\n", st.Name.Name, visMarker))
	sb.WriteString(fmt.Sprintf("*Line %d*\n\n", line))

	if st.Doc != nil {
		docText := extractDocString(st.Doc, src)
		if docText != "" {
			sb.WriteString(docText + "\n\n")
		}
	}

	if len(st.Fields) > 0 {
		sb.WriteString("**Fields:**\n")
		for _, f := range st.Fields {
			typeName := "any"
			if f.Type != nil {
				typeName = formatTypeName(f.Type)
			}
			sb.WriteString(fmt.Sprintf("- `%s: %s`\n", f.Name.Name, typeName))
		}
		sb.WriteString("\n")
	}
}

// renderEnum renders documentation for an enum.
func renderEnum(sb *strings.Builder, en *ast.EnumDecl, src []byte) {
	// Line number
	line := en.Span.Start.Line

	// Enum name with type params
	enumName := en.Name.Name
	if len(en.TypeParams) > 0 {
		var tps []string
		for _, tp := range en.TypeParams {
			tpStr := tp.Name.Name
			if len(tp.Bounds) > 0 {
				var bounds []string
				for _, b := range tp.Bounds {
					bounds = append(bounds, b.Name)
				}
				tpStr += ": " + strings.Join(bounds, " + ")
			}
			tps = append(tps, tpStr)
		}
		enumName += "<" + strings.Join(tps, ", ") + ">"
	}

	// Visibility marker
	visMarker := ""
	if !en.Pub {
		visMarker = " *(private)*"
	}

	sb.WriteString(fmt.Sprintf("### `%s`%s\n", enumName, visMarker))
	sb.WriteString(fmt.Sprintf("*Line %d*\n\n", line))

	if en.Doc != nil {
		docText := extractDocString(en.Doc, src)
		if docText != "" {
			sb.WriteString(docText + "\n\n")
		}
	}

	if len(en.Variants) > 0 {
		sb.WriteString("**Variants:**\n")
		for _, v := range en.Variants {
			if v.Type != nil {
				sb.WriteString(fmt.Sprintf("- `%s(%s)`\n", v.Name.Name, formatTypeName(v.Type)))
			} else {
				sb.WriteString(fmt.Sprintf("- `%s`\n", v.Name.Name))
			}
		}
		sb.WriteString("\n")
	}
}

// formatFuncSig formats a function signature (simple version for method lists).
func formatFuncSig(fn *ast.FuncDecl) string {
	var params []string
	for _, p := range fn.Params {
		typeName := "any"
		if p.Type != nil {
			typeName = formatTypeName(p.Type)
		}
		params = append(params, fmt.Sprintf("%s: %s", p.Name.Name, typeName))
	}

	sig := fmt.Sprintf("%s(%s)", fn.Name.Name, strings.Join(params, ", "))
	if fn.RetType != nil {
		sig += " -> " + formatTypeName(fn.RetType)
	}
	return sig
}

// formatFuncSigFull formats a function signature with type params and param modes.
func formatFuncSigFull(fn *ast.FuncDecl) string {
	// Type parameters
	typeParams := ""
	if len(fn.TypeParams) > 0 {
		var tps []string
		for _, tp := range fn.TypeParams {
			tpStr := tp.Name.Name
			if len(tp.Bounds) > 0 {
				var bounds []string
				for _, b := range tp.Bounds {
					bounds = append(bounds, b.Name)
				}
				tpStr += ": " + strings.Join(bounds, " + ")
			}
			tps = append(tps, tpStr)
		}
		typeParams = "<" + strings.Join(tps, ", ") + ">"
	}

	// Parameters with modes
	var params []string
	for _, p := range fn.Params {
		typeName := "any"
		if p.Type != nil {
			typeName = formatTypeName(p.Type)
		}
		paramStr := ""
		switch p.Mode {
		case ast.ParamRef:
			paramStr = "ref "
		case ast.ParamInout:
			paramStr = "inout "
		}
		paramStr += p.Name.Name + ": " + typeName
		if p.Default != nil {
			paramStr += " = ..."
		}
		params = append(params, paramStr)
	}

	sig := fmt.Sprintf("%s%s(%s)", fn.Name.Name, typeParams, strings.Join(params, ", "))
	if fn.RetType != nil {
		sig += " -> " + formatTypeName(fn.RetType)
	}
	return sig
}

// formatTypeName formats a TypeName including generics.
func formatTypeName(t *ast.TypeName) string {
	if t == nil {
		return "any"
	}
	if len(t.Params) > 0 {
		var params []string
		for _, p := range t.Params {
			params = append(params, formatTypeName(p))
		}
		return t.Name + "<" + strings.Join(params, ", ") + ">"
	}
	return t.Name
}

// extractDocString extracts docstring text from a StrLit.
func extractDocString(doc *ast.StrLit, src []byte) string {
	if doc.Value != "" {
		return strings.TrimSpace(doc.Value)
	}
	// Extract from source using span
	if doc.Span.Start.Byte > 0 && doc.Span.End.Byte > doc.Span.Start.Byte {
		text := string(src[doc.Span.Start.Byte:doc.Span.End.Byte])
		// Remove triple quotes
		text = strings.TrimPrefix(text, "\"\"\"")
		text = strings.TrimSuffix(text, "\"\"\"")
		return strings.TrimSpace(text)
	}
	return ""
}
