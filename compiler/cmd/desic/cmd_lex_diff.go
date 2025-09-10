package main

import (
	"os"
	"strings"

	"github.com/desilang/desi/compiler/internal/lexbridge"
	"github.com/desilang/desi/compiler/internal/term"
)

/* ---------- lex-diff (Go vs Desi) ---------- */

func cmdLexDiff(args []string) int {
	// parse flags: [--limit=N] [--verbose] <file>
	var limitStr string
	var verbose bool
	var file string
	for i := 0; i < len(args); i++ {
		s := args[i]
		if strings.HasPrefix(s, "--limit=") {
			limitStr = strings.TrimPrefix(s, "--limit=")
			continue
		}
		if s == "--verbose" {
			verbose = true
			continue
		}
		if !strings.HasPrefix(s, "-") && file == "" {
			file = s
			continue
		}
		if strings.HasPrefix(s, "-") {
			term.Eprintln("usage: desic lex-diff [--limit=N] [--verbose] <file.desi>")
			return 2
		}
	}
	if file == "" {
		term.Eprintln("usage: desic lex-diff [--limit=N] [--verbose] <file.desi>")
		return 2
	}
	limit := 0
	if limitStr != "" {
		// parse int; ignore error silently → 0 (no limit)
		n := 0
		for _, r := range limitStr {
			if r < '0' || r > '9' {
				n = 0
				break
			}
			n = n*10 + int(r-'0')
		}
		limit = n
	}

	data, err := os.ReadFile(file)
	if err != nil {
		term.Eprintf("read %s: %v\n", file, err)
		return 1
	}

	// Get Desi NDJSON via bridge (no stale fallback)
	raw, rerr := lexbridge.BuildAndRunRaw(file, false, verbose)
	if rerr != nil {
		term.Eprintf("lex-diff bridge: %v\n", rerr)
		return 1
	}
	nd := lexbridge.ConvertRawToNDJSON(raw, true)

	rows, derr := lexbridge.BuildLexDiff(string(data), nd)
	if derr != nil {
		term.Eprintf("ndjson parse warning: %v\n", derr)
	}
	term.Printf("%s", lexbridge.FormatDiff(rows, limit))
	return 0
}
