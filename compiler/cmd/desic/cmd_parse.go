package main

import (
  "fmt"
  "os"
  "reflect"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/build"
  "github.com/desilang/desi/compiler/internal/lexbridge"
  "github.com/desilang/desi/compiler/internal/parsebridge"
  "github.com/desilang/desi/compiler/internal/term"
)

/* ---------- parse ---------- */

func cmdParse(args []string) int {
  // Accept:
  //   desic parse
  //     [--use-desi-lexer]
  //     [--use-desi-parser] [--parsebridge-bin <path>]
  //     [--use-desi-parser-auto]
  //     [--keep-bridge-tmp] [--bridge-verbose]
  //     [--dump-go] [--dump-spans] [--dump-json]
  //     [--read-json <path>]
  //     <file.desi>
  useDesiLexer := false
  useDesiParser := false
  useDesiParserAuto := false
  parsebridgeBin := ""
  keepTmp := false
  verbose := false
  dumpGo := false
  dumpSpans := false
  dumpJSON := false
  readJSON := ""
  var file string

  usage := func() int {
    term.Eprintln("usage: desic parse [--use-desi-lexer] [--use-desi-parser] [--use-desi-parser-auto] [--parsebridge-bin <path>] [--keep-bridge-tmp] [--bridge-verbose] [--dump-go] [--dump-spans] [--dump-json] [--read-json <path>] <file.desi>")
    return 2
  }

  // simple argv parse
  for i := 0; i < len(args); i++ {
    s := args[i]
    switch {
    case s == "--use-desi-lexer":
      useDesiLexer = true
    case s == "--use-desi-parser":
      useDesiParser = true
    case s == "--use-desi-parser-auto":
      useDesiParserAuto = true
    case strings.HasPrefix(s, "--parsebridge-bin="):
      parsebridgeBin = strings.TrimPrefix(s, "--parsebridge-bin=")
    case s == "--parsebridge-bin":
      if i+1 >= len(args) {
        return usage()
      }
      parsebridgeBin = args[i+1]
      i++
    case s == "--keep-bridge-tmp":
      keepTmp = true
    case s == "--bridge-verbose":
      verbose = true
    case s == "--dump-go":
      dumpGo = true
    case s == "--dump-spans":
      dumpSpans = true
    case s == "--dump-json":
      dumpJSON = true
    case strings.HasPrefix(s, "--read-json="):
      readJSON = strings.TrimPrefix(s, "--read-json=")
    case s == "--read-json":
      if i+1 >= len(args) {
        return usage()
      }
      readJSON = args[i+1]
      i++
    case !strings.HasPrefix(s, "-") && file == "":
      file = s
    case strings.HasPrefix(s, "-"):
      return usage()
    }
  }

  // If --read-json is used, bypass parsing and reconstruct the AST.
  if strings.TrimSpace(readJSON) != "" {
    data, err := os.ReadFile(readJSON)
    if err != nil {
      term.Eprintf("error: read %s: %v\n", readJSON, err)
      return 1
    }
    f, err := ast.UnmarshalFileJSON(data)
    if err != nil {
      term.Eprintf("error: %v\n", err)
      return 1
    }
    return outputParsed(f, dumpGo, dumpSpans, dumpJSON)
  }

  if file == "" {
    return usage()
  }

  // Desi parser modes (external and/or auto-build).
  if useDesiParser || useDesiParserAuto {
    // Try external bridge first if requested.
    if useDesiParser {
      js, err := parsebridge.Run(file, parsebridgeBin, verbose)
      if err == nil {
        f, uerr := ast.UnmarshalFileJSON(js)
        if uerr != nil {
          term.Eprintf("error: parsebridge produced invalid AST JSON: %v\n", uerr)
          return 1
        }
        return outputParsed(f, dumpGo, dumpSpans, dumpJSON)
      }
      // If it's not installed and auto mode is allowed, fall through.
      if _, ok := err.(parsebridge.NotInstalledError); !ok || !useDesiParserAuto {
        if _, isNI := err.(parsebridge.NotInstalledError); isNI {
          term.Eprintln("error: Desi parser bridge not installed.")
          term.Eprintln("hint: put a 'desi-parsebridge' binary on your PATH or pass --parsebridge-bin <path>")
          term.Eprintln("      the tool should print AST JSON compatible with ast.MarshalFileJSON")
        } else {
          term.Eprintf("error: %v\n", err)
        }
        return 1
      }
      // else: NotInstalledError and auto is enabled → continue to auto build.
    }

    // Auto-build bridge path.
    if useDesiParserAuto {
      js, err := parsebridge.BuildAndRunJSON(file, keepTmp, verbose)
      if err != nil {
        switch e := err.(type) {
        case parsebridge.ParserSourcesMissingError:
          term.Eprintln("error: " + e.Error())
          term.Eprintln("hint: add a Desi parser at examples/compiler/desi/parser.desi implementing:")
          term.Eprintln("      def parse_to_json(path: str) -> str")
          return 1
        default:
          term.Eprintf("error: %v\n", err)
          return 1
        }
      }
      f, uerr := ast.UnmarshalFileJSON(js)
      if uerr != nil {
        term.Eprintf("error: parsebridge produced invalid AST JSON: %v\n", uerr)
        return 1
      }
      return outputParsed(f, dumpGo, dumpSpans, dumpJSON)
    }

    // Shouldn’t reach here, but just in case.
    term.Eprintln("error: Desi parser bridge not installed.")
    term.Eprintln("hint: put a 'desi-parsebridge' binary on your PATH or pass --parsebridge-bin <path>")
    term.Eprintln("      or use --use-desi-parser-auto to build from examples/compiler/desi/parser.desi")
    return 1
  }

  // Otherwise: Go-lexer or Desi-lexer (Stage-1) paths.
  f, errs := build.ResolveAndParseMaybeDesi(file, useDesiLexer, keepTmp, verbose)
  if len(errs) > 0 {
    for _, e := range errs {
      // Pretty-print lexbridge LEXERR diagnostics if present
      if pretty := lexbridge.RenderLexbridgeErrorPretty(e, file, nil); pretty != "" {
        term.Eprintf("%s", pretty)
      } else {
        term.Eprintf("%v\n", e)
      }
    }
    return 1
  }

  return outputParsed(f, dumpGo, dumpSpans, dumpJSON)
}

func outputParsed(f *ast.File, dumpGo, dumpSpans, dumpJSON bool) int {
  if dumpGo {
    fmt.Printf("%#v\n", f)
    return 0
  }
  if dumpSpans {
    dumpASTSpans(f)
    return 0
  }
  if dumpJSON {
    js, err := ast.MarshalFileJSON(f)
    if err != nil {
      term.Eprintf("error: %v\n", err)
      return 1
    }
    term.Printf("%s\n", js)
    return 0
  }
  out := ast.DumpFile(f)
  term.Printf("%s", out)
  return 0
}

/* ---------- span dumper (reflection-based) ---------- */

func dumpASTSpans(root any) {
  var (
    spanType = reflect.TypeOf(ast.Span{})
  )

  var walk func(v reflect.Value, indent int)

  printNode := func(v reflect.Value, indent int) {
    t := v.Type()
    name := t.Name()
    var hint string
    if f := v.FieldByName("Name"); f.IsValid() && f.Kind() == reflect.String {
      if s := f.String(); s != "" {
        hint = fmt.Sprintf(" %q", s)
      }
    } else if f := v.FieldByName("Op"); f.IsValid() && f.Kind() == reflect.String {
      if s := f.String(); s != "" {
        hint = fmt.Sprintf(" %q", s)
      }
    }
    var spanStr string
    if sf := v.FieldByName("Span"); sf.IsValid() && sf.Type() == spanType {
      s := sf.Interface().(ast.Span)
      if (s.Start.Line | s.Start.Col | s.End.Line | s.End.Col) != 0 {
        spanStr = fmt.Sprintf(" [L%d:%d–L%d:%d]", s.Start.Line, s.Start.Col, s.End.Line, s.End.Col)
      }
    }
    term.Printf("%s%s%s%s\n", strings.Repeat("  ", indent), name, hint, spanStr)
  }

  walk = func(v reflect.Value, indent int) {
    if !v.IsValid() {
      return
    }
    for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
      if v.IsNil() {
        return
      }
      v = v.Elem()
    }
    switch v.Kind() {
    case reflect.Slice, reflect.Array:
      for i := 0; i < v.Len(); i++ {
        walk(v.Index(i), indent)
      }
    case reflect.Struct:
      if v.FieldByName("Span").IsValid() || v.Type().Name() == "File" || v.Type().Name() == "FuncDecl" || v.Type().Name() == "PackageDecl" || v.Type().Name() == "ImportDecl" {
        printNode(v, indent)
        indent++
      }
      for i := 0; i < v.NumField(); i++ {
        walk(v.Field(i), indent)
      }
    default:
    }
  }

  walk(reflect.ValueOf(root), 0)
}
