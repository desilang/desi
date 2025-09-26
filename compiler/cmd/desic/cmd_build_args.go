package main

import (
	"os"
	"strings"
)

/* ---------- build (flags anywhere) ---------- */

type buildArgs struct {
	// compile/link behavior
	noCC       bool
	ccBin      string   // --cc-bin (alias: --cc)
	ccArgs     []string // --cc-arg (repeatable)
	runtimeDir string   // --runtime-dir

	// outputs
	out  string // executable name (no dir); defaults to entry basename
	file string // entry .desi file

	// diagnostics / behavior
	werr          bool // --Werror treats warnings as errors
	useDesi       bool // --use-desi-lexer: route through lexbridge (Go parser)
	verbose       bool // --verbose: pass to bridges
	keepBridgeTmp bool // --keep-bridge-tmp: retain gen/tmp artifacts

	// Desi parser bridge options
	useDesiParser     bool   // --use-desi-parser (external bridge)
	useDesiParserAuto bool   // --use-desi-parser-auto (auto build)
	parsebridgeBin    string // --parsebridge-bin <path> (for external)
}

func parseBuildArgs(argv []string) (buildArgs, error) {
	var a buildArgs
	i := 0
	for i < len(argv) {
		s := argv[i]
		if s == "--" {
			i++
			break
		}

		switch {
		// emit C only
		case s == "--no-cc":
			a.noCC = true
			i++
			continue

		// compiler binary: --cc-bin=clang  (alias: --cc=clang)
		case strings.HasPrefix(s, "--cc-bin="):
			a.ccBin = s[len("--cc-bin="):]
			i++
			continue
		case s == "--cc-bin":
			if i+1 >= len(argv) {
				return a, flagErrHelp
			}
			a.ccBin = argv[i+1]
			i += 2
			continue
		// backward-compat alias
		case strings.HasPrefix(s, "--cc="):
			a.ccBin = s[len("--cc="):]
			i++
			continue
		case s == "--cc":
			if i+1 >= len(argv) {
				return a, flagErrHelp
			}
			a.ccBin = argv[i+1]
			i += 2
			continue

		// pass-through cc arg (repeatable)
		case strings.HasPrefix(s, "--cc-arg="):
			a.ccArgs = append(a.ccArgs, s[len("--cc-arg="):])
			i++
			continue
		case s == "--cc-arg":
			if i+1 >= len(argv) {
				return a, flagErrHelp
			}
			a.ccArgs = append(a.ccArgs, argv[i+1])
			i += 2
			continue

		// runtime dir override
		case strings.HasPrefix(s, "--runtime-dir="):
			a.runtimeDir = s[len("--runtime-dir="):]
			i++
			continue
		case s == "--runtime-dir":
			if i+1 >= len(argv) {
				return a, flagErrHelp
			}
			a.runtimeDir = argv[i+1]
			i += 2
			continue

		// output executable name (no directory)
		case strings.HasPrefix(s, "--out="):
			a.out = s[len("--out="):]
			i++
			continue
		case s == "--out":
			if i+1 >= len(argv) {
				return a, flagErrHelp
			}
			a.out = argv[i+1]
			i += 2
			continue

		// diagnostics / behavior
		case s == "--Werror" || s == "--werror":
			a.werr = true
			i++
			continue
		case s == "--use-desi-lexer":
			a.useDesi = true
			i++
			continue
		case s == "--verbose":
			a.verbose = true
			i++
			continue
		case s == "--keep-bridge-tmp":
			a.keepBridgeTmp = true
			i++
			continue

		// Desi parser bridge flags
		case s == "--use-desi-parser":
			a.useDesiParser = true
			i++
			continue
		case s == "--use-desi-parser-auto":
			a.useDesiParserAuto = true
			i++
			continue
		case strings.HasPrefix(s, "--parsebridge-bin="):
			a.parsebridgeBin = s[len("--parsebridge-bin="):]
			i++
			continue
		case s == "--parsebridge-bin":
			if i+1 >= len(argv) {
				return a, flagErrHelp
			}
			a.parsebridgeBin = argv[i+1]
			i += 2
			continue
		}

		// first non-flag is entry file
		if !strings.HasPrefix(s, "-") && a.file == "" {
			a.file = s
			i++
			continue
		}
		if strings.HasPrefix(s, "-") {
			return a, flagErrHelp
		}
		i++
	}

	// If "--" was used, remaining args: first non-flag is file.
	for i < len(argv) && a.file == "" {
		if !strings.HasPrefix(argv[i], "-") {
			a.file = argv[i]
		}
		i++
	}

	if a.file == "" {
		return a, flagErrHelp
	}
	return a, nil
}

// local minimal flag.ErrHelp replacement to avoid importing flag
var flagErrHelp = os.ErrInvalid
