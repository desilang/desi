package main

import (
	"flag"
	"strings"

	"github.com/desilang/desi/compiler/internal/lexbridge"
	"github.com/desilang/desi/compiler/internal/term"
)

/* ---------- EXPERIMENTAL: run Desi lexer (compiler.desi.lexer) ---------- */

type lexDesiArgs struct {
	file    string
	keepTmp bool
	format  string // "raw" | "ndjson" | "pretty"
	verbose bool
}

func parseLexDesiArgs(argv []string) (lexDesiArgs, error) {
	var a lexDesiArgs
	a.format = "raw"
	i := 0
	for i < len(argv) {
		s := argv[i]
		switch {
		case s == "--keep-tmp":
			a.keepTmp = true
			i++
			continue
		case s == "--verbose":
			a.verbose = true
			i++
			continue
		case strings.HasPrefix(s, "--format="):
			a.format = strings.TrimPrefix(s, "--format=")
			if a.format != "raw" && a.format != "ndjson" && a.format != "pretty" {
				return a, flag.ErrHelp
			}
			i++
			continue
		}
		if !strings.HasPrefix(s, "-") && a.file == "" {
			a.file = s
			i++
			continue
		}
		if strings.HasPrefix(s, "-") {
			return a, flag.ErrHelp
		}
		i++
	}
	if a.file == "" {
		return a, flag.ErrHelp
	}
	return a, nil
}

func cmdLexDesiRun(args []string) int {
	a, err := parseLexDesiArgs(args)
	if err != nil {
		term.Eprintln("usage: desic lex-desi [--keep-tmp] [--format=raw|ndjson|pretty] [--verbose] <file.desi>")
		return 2
	}

	raw, rerr := lexbridge.BuildAndRunRaw(a.file, a.keepTmp, a.verbose)
	if rerr != nil {
		term.Eprintf("lex-desi: %v\n", rerr)
		return 1
	}

	switch a.format {
	case "raw":
		lexbridge.MirrorErrsToStderr(raw)
		term.Printf("%s", raw)
	case "ndjson":
		nd := lexbridge.ConvertRawToNDJSON(raw, true)
		term.Printf("%s", nd)
	case "pretty":
		nd := lexbridge.ConvertRawToNDJSON(raw, true)
		toks, perr := lexbridge.ParseNDJSON(strings.NewReader(nd))
		if perr != nil {
			term.Eprintf("ndjson parse warning: %v\n", perr)
		}
		pretty := lexbridge.DebugFormat(toks, 0)
		term.Printf("%s", pretty)
	default:
		term.Printf("%s", raw)
	}
	return 0
}
