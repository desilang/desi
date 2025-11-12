package main

import (
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/format"
	"github.com/desilang/desi/compiler/internal/term"
)

var (
	// NEW:
	errorFormat = flag.String("error-format", "human", "error format: human|json")
	colorFlag   = flag.String("color", "auto", "color: auto|always|never")
)

var (
	writeInPlace = flag.Bool("w", false, "write result to (source) files instead of stdout")
	listOnly     = flag.Bool("l", false, "list files whose formatting differs")
	quiet        = flag.Bool("q", false, "quiet (suppress \"wrote ...\" messages with -w)")
)

func main() {
	flag.Parse()
	// Apply global diagnostics render preferences
	var cm diag.ColorMode
	switch strings.ToLower(*colorFlag) {
	case "always":
		cm = diag.Always
	case "never":
		cm = diag.Never
	default:
		cm = diag.Auto
	}
	ef := strings.ToLower(*errorFormat)
	diag.SetGlobalRender(ef, cm)
	jsonMode := ef == "json"
	if jsonMode {
		diag.BeginJSONCapture()
	}

	args := flag.Args()
	if len(args) == 0 {
		usage()
		term.Flush()
		if jsonMode {
			if err := diag.EndJSONCapture(os.Stderr); err != nil {
				term.Eprintln("desifmt: json flush:", err)
			}
		}
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
				term.Eprintln("desifmt:", err)
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

	term.Flush()
	if jsonMode {
		if err := diag.EndJSONCapture(os.Stderr); err != nil {
			term.Eprintln("desifmt: json flush:", err)
		}
	}
	if hadArgErr {
		os.Exit(2)
	}
	if hadDiag {
		os.Exit(1)
	}
	os.Exit(0)
}

func usage() {
	term.Println("usage: desifmt [flags] path-or-'-' ...")
	flag.PrintDefaults()
}

// formatStdin / formatFile remain unchanged except RenderTTY honors global mode.

func formatStdin() (bool, []diag.Diagnostic) {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		term.Eprintln("desifmt:", err)
		return false, nil
	}

	out, diags := format.FormatBytes(src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		return false, diags
	}

	if *listOnly {
		// Listing is non-error info → stdout.
		term.Println("<stdin>")
		return true, nil
	}

	if *writeInPlace {
		term.Eprintln("desifmt: -w ignored on stdin")
		return false, nil
	}

	term.Write(os.Stdout, out)
	return true, nil
}

func formatFile(path string) (bool, []diag.Diagnostic) {
	f, err := os.Open(path)
	if err != nil {
		term.Eprintln("desifmt:", err)
		return false, nil
	}
	defer func() { _ = f.Close() }()

	src, err := io.ReadAll(f)
	if err != nil {
		term.Eprintln("desifmt:", err)
		return false, nil
	}

	out, diags := format.FormatBytes(src)
	if len(diags) > 0 {
		for _, d := range diags {
			d.RenderTTY(os.Stderr, diag.Theme{})
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
		if bytesEqual(src, out) {
			if !*quiet {
				term.Println("formatted:", path, "(no changes)")
			}
			return true, nil
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			term.Eprintln("desifmt:", err)
			return false, nil
		}
		if !*quiet {
			term.Println("wrote:", path)
		}
		return true, nil
	}

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
