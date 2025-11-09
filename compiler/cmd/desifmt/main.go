package main

import (
	"flag"
	"io"
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
	quiet        = flag.Bool("q", false, "quiet (suppress \"wrote ...\" messages with -w)")
)

func main() {
	flag.Parse()
	defer term.Flush()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	hadDiag := false
	hadArgErr := false

	for _, a := range args {
		if a == "-" {
			ok, diags := formatStdin()
			if !ok && len(diags) > 0 {
				hadDiag = true
			} else if !ok {
				hadArgErr = true
			}
			continue
		}

		stat, err := os.Stat(a)
		if err != nil {
			term.Eprintln("desifmt:", err)
			hadArgErr = true
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
				ok, diags := formatFile(path)
				if !ok && len(diags) > 0 {
					hadDiag = true
				} else if !ok {
					hadArgErr = true
				}
				return nil
			})
			if err != nil {
				term.Eprintln("desifmt walk:", err)
				hadArgErr = true
			}
			continue
		}

		ok, diags := formatFile(a)
		if !ok && len(diags) > 0 {
			hadDiag = true
		} else if !ok {
			hadArgErr = true
		}
	}

	if hadArgErr {
		os.Exit(2)
	}
	if hadDiag {
		os.Exit(1)
	}
}

func usage() {
	term.Eprintln("usage: desifmt [-w|-l|-q] <file|dir|-> [...]")
}

func formatStdin() (bool, []diag.Diagnostic) {
	// With '-', ignore -w and -l; always print to stdout.
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		term.Eprintln("desifmt:", err)
		return false, nil
	}
	out, diags := format.FormatBytes(src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{Color: false})
		}
		// Still return false so caller sets exit=1.
		return false, diags
	}
	term.Write(os.Stdout, out)
	return true, nil
}

func formatFile(path string) (bool, []diag.Diagnostic) {
	src, err := os.ReadFile(path)
	if err != nil {
		term.Eprintln("desifmt:", err)
		return false, nil
	}
	out, diags := format.FormatBytes(src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{Color: false})
		}
		return false, diags
	}

	if *listOnly {
		if !bytesEqual(src, out) {
			// Listing is non-error info → stdout.
			term.Println(path)
		}
		return true, nil
	}

	if *writeInPlace {
		if !bytesEqual(src, out) {
			if err := os.WriteFile(path, out, 0o644); err != nil {
				term.Eprintln("desifmt:", err)
				return false, nil
			}
			if !*quiet {
				term.Eprintln("wrote", path)
			}
		}
		return true, nil
	}

	// Default: write formatted bytes to stdout.
	term.Write(os.Stdout, out)
	return true, nil
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
