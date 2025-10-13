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

	changed := false
	for _, a := range args {
		stat, err := os.Stat(a)
		if err != nil {
			term.Eprintln("desifmt:", err)
			continue
		}
		if stat.IsDir() {
			filepath.WalkDir(a, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || filepath.Ext(path) != ".desi" {
					return nil
				}
				ok, err := formatOne(path)
				if err != nil {
					term.Eprintln("desifmt:", err)
				}
				if !ok {
					changed = true
				}
				return nil
			})
		} else {
			ok, err := formatOne(a)
			if err != nil {
				term.Eprintln("desifmt:", err)
			}
			if !ok {
				changed = true
			}
		}
	}
	// On -l with no changes, be silent.
	term.Flush()
}

func formatOne(path string) (alreadyFormatted bool, err error) {
	// TODO: parse + pretty-print AST; for now, just simulate "already formatted".
	if *listOnly {
		// print path if it WOULD change; stub says nothing changes
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
