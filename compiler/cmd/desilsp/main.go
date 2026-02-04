// desilsp is the Desi Language Server.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/desilang/desi/compiler/internal/lsp"
)

var version = "0.1.0"

func main() {
	logFile := flag.String("log", "", "Log file path (optional)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("desilsp %s\n", version)
		return
	}

	server := lsp.New(*logFile)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "desilsp error: %v\n", err)
		os.Exit(1)
	}
}
