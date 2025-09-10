package main

import (
	"os"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexbridge"
	"github.com/desilang/desi/compiler/internal/parser"
	"github.com/desilang/desi/compiler/internal/term"
)

/* ---------- parse ---------- */

func cmdParse(args []string) int {
	// Accept: parse [--use-desi-lexer] [--verbose] <file>
	useDesi := false
	verbose := false
	var file string
	for _, s := range args {
		switch {
		case s == "--use-desi-lexer":
			useDesi = true
		case s == "--verbose":
			verbose = true
		case !strings.HasPrefix(s, "-") && file == "":
			file = s
		case strings.HasPrefix(s, "-"):
			term.Eprintln("usage: desic parse [--use-desi-lexer] [--verbose] <file.desi>")
			return 2
		}
	}
	if file == "" {
		term.Eprintln("usage: desic parse [--use-desi-lexer] [--verbose] <file.desi>")
		return 2
	}

	if useDesi {
		// Stage-1 path: Desi lexer -> adapter -> Go parser
		src, err := lexbridge.NewSourceFromFileOpts(file, false /*keepTmp*/, verbose /*verbose*/)
		if err != nil {
			term.Eprintf("desi-lexer adapter: %v\n", err)
			return 1
		}
		ts, ok := src.(parser.TokenSource)
		if !ok {
			term.Eprintln("desi-lexer adapter: internal type mismatch (value does not satisfy parser.TokenSource)")
			return 1
		}
		p := parser.NewFromSource(ts)
		f, perr := p.ParseFile()
		if perr != nil {
			term.Eprintf("parse: %v\n", perr)
			return 1
		}
		out := ast.DumpFile(f)
		term.Printf("%s", out)
		return 0
	}

	// Legacy Go-lexer path
	data, err := os.ReadFile(file)
	if err != nil {
		term.Eprintf("read %s: %v\n", file, err)
		return 1
	}
	p := parser.New(string(data))
	f, perr := p.ParseFile()
	if perr != nil {
		term.Eprintf("parse: %v\n", perr)
		return 1
	}
	out := ast.DumpFile(f)
	term.Printf("%s", out)
	return 0
}
