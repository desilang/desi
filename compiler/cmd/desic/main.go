package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/desilang/desi/compiler/internal/version"
)

func usage() {
	fmt.Fprintf(os.Stderr, "desic — Desi compiler (Stage-0)\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n  desic <command> [args]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  version       Print version\n")
	fmt.Fprintf(os.Stderr, "  help          Show this help\n")
}

func main() {
	flag.Usage = usage
	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version.String())
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
