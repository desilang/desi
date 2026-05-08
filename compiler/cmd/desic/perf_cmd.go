package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/term"
)

// perfCmd runs the performance advisor on one or more .desi files.
//
// Usage:
//   desic perf [--level=relaxed|default|strict] <file.desi...>
//   desic perf                                   (uses desi.mod entry)
//
// Exits 0 if no advisor warnings, 1 if warnings found, 2 on error.
func perfCmd(argv []string) int {
	level := "default"
	var files []string

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case strings.HasPrefix(a, "--level="):
			level = strings.TrimPrefix(a, "--level=")
		case a == "--level":
			if i+1 < len(argv) {
				i++
				level = argv[i]
			} else {
				term.Eprintln("perf: missing value for --level")
				return 2
			}
		case strings.HasPrefix(a, "-"):
			// skip render flags
		default:
			files = append(files, a)
		}
	}

	// Validate level
	switch level {
	case "relaxed", "default", "strict":
		// ok
	default:
		term.Eprintln("perf: invalid --level:", level, "(expected relaxed|default|strict)")
		return 2
	}

	// If no files given, try desi.mod
	if len(files) == 0 {
		m, _, ok := loadManifestOrFail("perf")
		if !ok {
			return 2
		}
		entry := m.EntryPath()
		if entry == "" {
			term.Eprintln("perf: no entry file in desi.mod")
			return 2
		}
		files = append(files, entry)
	}

	totalWarnings := 0
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			term.Eprintln("perf: file not found:", file)
			return 2
		}

		src, err := os.ReadFile(file)
		if err != nil {
			term.Eprintln("perf: cannot read:", err)
			return 2
		}

		mod, pdiags := parse.ParseFile(file, src)
		if len(pdiags) > 0 {
			for _, d := range pdiags {
				d.RenderTTY(os.Stderr, diag.Theme{Color: false})
			}
			return 2
		}

		// Run the advisor
		warnings := check.RunPerfAdvisor(mod, level)
		totalWarnings += len(warnings)

		if len(warnings) == 0 {
			term.Println("perf:", filepath.Base(file), "— ok")
			continue
		}

		for _, w := range warnings {
			w.RenderTTY(os.Stderr, diag.Theme{Color: false})
		}
	}

	term.Println("")
	if totalWarnings == 0 {
		term.Println("✓ No performance issues detected")
		return 0
	}
	term.Println("⚠", totalWarnings, "performance warning(s) found")
	return 1
}
