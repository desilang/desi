package main

import (
	"flag"
	"fmt"
	"os"

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
	default:
		eprintf("unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
