package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/format"
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
		usage()
		os.Exit(2)
	}

	ok := true
	for _, a := range args {
		if a == "-" {
			ok = formatStdin() && ok
			continue
		}
		stat, err := os.Stat(a)
		if err != nil {
			term.Eprintln("desifmt:", err)
			ok = false
			continue
		}
		if stat.IsDir() {
			err := filepath.WalkDir(a, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || filepath.Ext(path) != ".desi" {
					return nil
				}
				if ok2 := formatFile(path); !ok2 {
					ok = false
				}
				return nil
			})
			if err != nil {
				term.Eprintln("desifmt walk:", err)
				ok = false
			}
		} else {
			if ok2 := formatFile(a); !ok2 {
				ok = false
			}
		}
	}
	if !ok {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: desifmt [-w|-l] <file|dir|-> [...]")
}

func formatStdin() bool {
	src, err := os.ReadFile("/dev/stdin")
	if err != nil {
		term.Eprintln("desifmt:", err)
		return false
	}
	out, diags := format.FormatBytes(src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{Color: false})
		}
		return false
	}
	if *writeInPlace {
		// not meaningful for stdin
		return true
	}
	term.Write(os.Stdout, out)
	return true
}

func formatFile(path string) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		term.Eprintln("desifmt:", err)
		return false
	}
	out, diags := format.FormatBytes(src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{Color: false})
		}
		return false
	}
	if *listOnly {
		if !bytesEqual(src, out) {
			term.Eprintln(path)
		}
		return true
	}
	if *writeInPlace {
		if !bytesEqual(src, out) {
			if err := os.WriteFile(path, out, 0644); err != nil {
				term.Eprintln("desifmt:", err)
				return false
			}
			term.Eprintln("wrote", path)
		}
		return true
	}
	// default: print to stdout
	term.Write(os.Stdout, out)
	return true
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
