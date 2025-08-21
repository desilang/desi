package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/desilang/desi/compiler/internal/lexer"
	"github.com/desilang/desi/compiler/internal/version"
)

func eprintf(format string, a ...any) { _, _ = fmt.Fprintf(os.Stderr, format, a...) }
func eprintln(a ...any)               { _, _ = fmt.Fprintln(os.Stderr, a...) }
func printf(format string, a ...any)  { _, _ = fmt.Printf(format, a...) }

func usage() {
	eprintln("desic — Desi compiler (Stage-0)")
	eprintln("")
	eprintln("Usage:")
	eprintln("  desic <command> [args]")
	eprintln("")
	eprintln("Commands:")
	eprintln("  version       Print version")
	eprintln("  help          Show this help")
	eprintln("  lex <file>    Lex a .desi file and print tokens")
}

func cmdLex(args []string) int {
	if len(args) != 1 {
		eprintln("usage: desic lex <file.desi>")
		return 2
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		eprintf("read %s: %v\n", args[0], err)
		return 1
	}
	lx := lexer.New(string(data))
	for {
		t := lx.Next()
		if t.Kind == lexer.TokEOF {
			printf("%d:%d  %s\n", t.Line, t.Col, t.Kind)
			break
		}
		lex := t.Lex
		// shorten long lexemes in output
		if len(lex) > 40 {
			lex = lex[:37] + "..."
		}
		if lex == "" {
			printf("%d:%d  %-8s\n", t.Line, t.Col, t.Kind)
		} else {
			printf("%d:%d  %-8s  %q\n", t.Line, t.Col, t.Kind, lex)
		}
	}
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
		printf("%s\n", version.String())
	case "help", "--help", "-h":
		usage()
	case "lex":
		os.Exit(cmdLex(os.Args[2:]))
	default:
		eprintf("unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
