package main

import "github.com/desilang/desi/compiler/internal/term"

func usage() {
	term.Eprintln("desic — Desi compiler (Stage-1)")
	term.Eprintln("")
	term.Eprintln("Usage:")
	term.Eprintln("  desic <command> [args]")
	term.Eprintln("")
	term.Eprintln("Commands:")
	term.Eprintln("  version                                   Print version")
	term.Eprintln("  help                                      Show this help")
	term.Eprintln("  lex <file>                                Lex a .desi file (Go lexer) and print tokens")
	term.Eprintln("  parse [--use-desi-lexer] [--verbose] <file>  Parse a .desi file and print AST outline")
	term.Eprintln("  build [--cc=clang] [--out=name] [--Werror] [--use-desi-lexer] [--verbose] <entry.desi>")
	term.Eprintln("                                             (flags may appear before or after the file)")
	term.Eprintln("  lex-desi [--keep-tmp] [--format=raw|ndjson|pretty] [--verbose] <file>")
	term.Eprintln("                                             EXPERIMENTAL: run Desi lexer (compiler.desi.lexer) and print tokens")
	term.Eprintln("  lex-diff [--limit=N] [--verbose] <file>   Compare Go vs Desi token streams (by index)")
	term.Eprintln("  lex-map [--verbose] <file>                Report Desi→Go kind mapping coverage on this file")
	term.Eprintln("")
	term.Eprintln("Notes:")
	term.Eprintln("  - Imports like 'foo.bar' resolve to 'foo/bar.desi' relative to the entry file’s dir.")
	term.Eprintln("  - Imports starting with 'std.' are ignored in Stage-0/1 (provided by runtime).")
	term.Eprintln("")
	term.Eprintln("Outputs:")
	term.Eprintln("  generated C:   gen/out/<basename>.c")
	term.Eprintln("  binary (if --cc): gen/out/<out|basename>")
}
