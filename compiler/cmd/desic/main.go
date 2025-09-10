package main

import (
	"flag"
	"os"

	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/version"
)

/* ---------- main ---------- */

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
		if len(os.Args) != 3 {
			term.Eprintln("usage: desic lex <file.desi>")
			os.Exit(2)
		}
		os.Exit(cmdLexDirect(os.Args[2]))
	case "parse":
		os.Exit(cmdParse(os.Args[2:]))
	case "build":
		os.Exit(cmdBuild(os.Args[2:]))
	case "lex-desi":
		if len(os.Args) < 3 {
			term.Eprintln("usage: desic lex-desi [--keep-tmp] [--format=raw|ndjson|pretty] [--verbose] <file.desi>")
			os.Exit(2)
		}
		os.Exit(cmdLexDesiRun(os.Args[2:]))
	case "lex-diff":
		if len(os.Args) < 3 {
			term.Eprintln("usage: desic lex-diff [--limit=N] [--verbose] <file.desi>")
			os.Exit(2)
		}
		os.Exit(cmdLexDiff(os.Args[2:]))
	case "lex-map":
		if len(os.Args) < 3 {
			term.Eprintln("usage: desic lex-map [--verbose] <file.desi>")
			os.Exit(2)
		}
		os.Exit(cmdLexMap(os.Args[2:]))
	default:
		term.Eprintf("unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
