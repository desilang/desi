package main

import (
	"flag"
	"os"

	"github.com/desilang/desi/compiler/internal/ast"
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
	term.Eprintln("  version          Print version")
	term.Eprintln("  help             Show this help")
	term.Eprintln("  lex <file>       Lex a .desi file and print tokens")
	term.Eprintln("  parse <file>     Parse a .desi file and print AST outline")
}

func cmdLex(args []string) int {
	if len(args) != 1 {
		term.Eprintln("usage: desic lex <file.desi>")
		return 2
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		term.Eprintf("read %s: %v\n", args[0], err)
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
		os.Exit(cmdLex(os.Args[2:]))
	case "parse":
		os.Exit(cmdParse(os.Args[2:]))
	default:
		term.Eprintf("unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
