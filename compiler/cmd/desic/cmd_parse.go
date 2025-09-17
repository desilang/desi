package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/build"
	"github.com/desilang/desi/compiler/internal/lexbridge"
	"github.com/desilang/desi/compiler/internal/term"
)

/* ---------- parse ---------- */

func cmdParse(args []string) int {
	// Accept:
	//   desic parse [--use-desi-lexer] [--keep-bridge-tmp] [--bridge-verbose] [--dump-go] [--dump-spans] [--dump-json] <file.desi>
	useDesi := false
	keepTmp := false
	verbose := false
	dumpGo := false
	dumpSpans := false
	dumpJSON := false
	var file string

	usage := func() int {
		term.Eprintln("usage: desic parse [--use-desi-lexer] [--keep-bridge-tmp] [--bridge-verbose] [--dump-go] [--dump-spans] [--dump-json] <file.desi>")
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
		case s == "--dump-spans":
			dumpSpans = true
		case s == "--dump-json":
			dumpJSON = true
		case !strings.HasPrefix(s, "-") && file == "":
			file = s
		case strings.HasPrefix(s, "-"):
			return usage()
		}
	}
	if file == "" {
		return usage()
	}

	// Choose Go-lexer or Desi-lexer bridge.
	f, errs := build.ResolveAndParseMaybeDesi(file, useDesi, keepTmp, verbose)
	if len(errs) > 0 {
		for _, e := range errs {
			if pretty := lexbridge.RenderLexbridgeErrorPretty(e, file, nil); pretty != "" {
				term.Eprintf("%s", pretty)
			} else {
				term.Eprintf("%v\n", e)
			}
		}
		return 1
	}

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
