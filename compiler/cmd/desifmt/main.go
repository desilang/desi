package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/desilang/desi/compiler/internal/term"
)

var (
	writeInPlace = flag.Bool("w", false, "write result to (source) files instead of stdout")
	listOnly     = flag.Bool("l", false, "list files whose formatting differs")
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		term.Eprintln("usage: desifmt [-w|-l] <files or dirs>")
		os.Exit(2)
	}

	for _, a := range args {
		stat, err := os.Stat(a)
		if err != nil {
			term.Eprintln("desifmt:", err)
			continue
		}
		if stat.IsDir() {
			if err := filepath.WalkDir(a, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					// propagate filesystem errors to terminate the walk
					return err
				}
				if d.IsDir() || filepath.Ext(path) != ".desi" {
					return nil
				}
				_, ferr := formatOne(path)
				return ferr
			}); err != nil {
				term.Eprintln("desifmt walk:", err)
			}
		} else {
			if _, err := formatOne(a); err != nil {
				term.Eprintln("desifmt:", err)
			}
		}
	}

	// On -l with no differences, be silent (stub never reports differences yet).
	term.Flush()
}

func formatOne(path string) (alreadyFormatted bool, err error) {
	// TODO: parse + pretty-print AST; for now, just simulate "already formatted".
	if *listOnly {
		// Would print the path if formatting would change; stub: nothing changes.
		return true, nil
	}
	if *writeInPlace {
		// stub: no changes written
		return true, nil
	}
	// print original to stdout as a placeholder
	b, err := os.ReadFile(path)
	if err != nil {
		return true, err
	}
	term.Write(os.Stdout, b)
	return true, nil
}
