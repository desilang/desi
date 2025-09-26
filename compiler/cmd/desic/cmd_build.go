package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/build"
	"github.com/desilang/desi/compiler/internal/cc"
	"github.com/desilang/desi/compiler/internal/check"
	cgen "github.com/desilang/desi/compiler/internal/codegen/c"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/lexbridge"
	"github.com/desilang/desi/compiler/internal/term"
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

	// NEW: Desi parser bridge options
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

		// NEW: Desi parser bridge flags
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

func usageBuild() {
	term.Eprintln("usage: desic build [flags] <entry.desi>")
	term.Eprintln("\nGeneral:")
	term.Eprintln("  --use-desi-lexer           use the self-hosted Desi lexer via bridge (Go parser)")
	term.Eprintln("  --use-desi-parser          use an external Desi parser bridge (AST JSON)")
	term.Eprintln("  --use-desi-parser-auto     auto-build + cache bridge from examples/compiler/desi/parser.desi")
	term.Eprintln("  --parsebridge-bin <path>   path to external parser bridge when using --use-desi-parser")
	term.Eprintln("  --keep-bridge-tmp          keep gen/tmp bridge artifacts (debugging)")
	term.Eprintln("  --verbose                  verbose bridge logging")
	term.Eprintln("  --Werror                   treat warnings as errors")
	term.Eprintln("\nC compile/link (enabled by default):")
	term.Eprintln("  --no-cc                    only emit C (skip compiling)")
	term.Eprintln("  --cc-bin=<cc>              choose compiler (clang/gcc/cl). Alias: --cc=<cc>")
	term.Eprintln("  --cc-arg=<flag>            pass through a flag to the C compiler (repeatable)")
	term.Eprintln("  --runtime-dir=<path>       override path to runtime/c (auto-detected otherwise)")
	term.Eprintln("  --out=<name>               output executable name (default: entry basename)")
}

func cmdBuild(args []string) int {
	a, err := parseBuildArgs(args)
	if err != nil {
		usageBuild()
		return 2
	}

	// Choose parsing path:
	// 1) Parser bridge (external or auto) if requested
	// 2) Else: Go/Desi lexer path (existing behavior)
	var (
		merged *ast.File
		perr   []error
	)
	if a.useDesiParser || a.useDesiParserAuto {
		merged, perr = build.ResolveAndParseWithParserBridge(
			a.file,
			a.useDesiParser,  // useExternal when true
			a.parsebridgeBin, // optional path for external
			a.keepBridgeTmp,
			a.verbose,
		)
	} else {
		merged, perr = build.ResolveAndParseMaybeDesi(a.file, a.useDesi, a.keepBridgeTmp, a.verbose)
	}

	if len(perr) > 0 {
		loaderErrs := 0
		loaderWarns := 0
		for _, e := range perr {
			// Pretty lexbridge errors if present; treat as errors.
			if pretty := lexbridge.RenderLexbridgeErrorPretty(e, guessErrFile(e.Error(), a.file), nil); pretty != "" {
				term.Eprintf("%s", pretty)
				loaderErrs++
				continue
			}
			// Typed diagnostics from loader: warnings (DW...) vs errors (everything else).
			type codedWithKey interface {
				Code() string
				Title() string
				Domain() string
				Key() string
			}
			if te, ok := e.(codedWithKey); ok && strings.TrimSpace(te.Code()) != "" {
				codeUp := strings.ToUpper(strings.TrimSpace(te.Code()))
				if strings.HasPrefix(codeUp, "DW") {
					term.Eprintf("warning[%s]: %s\n", te.Code(), te.Title())
					if h := lookupHelp(te.Domain(), te.Key()); strings.TrimSpace(h) != "" {
						term.Eprintf("help: %s\n", h)
					}
					loaderWarns++
				} else {
					term.Eprintf("error[%s]: %s\n", te.Code(), te.Title())
					if h := lookupHelp(te.Domain(), te.Key()); strings.TrimSpace(h) != "" {
						term.Eprintf("help: %s\n", h)
					}
					loaderErrs++
				}
				continue
			}
			// Fallback: unknown type → error.
			term.Eprintf("error: %v\n", e)
			loaderErrs++
		}
		if loaderErrs > 0 || (a.werr && loaderWarns > 0) {
			term.Eprintf("summary: %d error(s), %d warning(s)\n", loaderErrs, loaderWarns)
			return 1
		}
		// If only warnings, continue to typecheck/codegen.
		term.Eprintf("summary: %d error(s), %d warning(s)\n", 0, loaderWarns)
	}

	// Typecheck
	info, errs, warns := cgenCheckFileShim(merged)

	// warnings
	for _, w := range warns {
		code := strings.TrimSpace(w.Code)
		if code != "" {
			term.Eprintf("warning[%s]: %s\n", code, w.Msg)
			if h := warnHelpFromCode(code); strings.TrimSpace(h) != "" {
				term.Eprintf("help: %s\n", h)
			}
		} else {
			term.Eprintf("warning: %s\n", w.Msg)
		}
	}

	// errors: prefer pretty span rendering when available
	for _, e := range errs {
		type codedWithKey interface {
			Code() string
			Title() string
			Domain() string
			Key() string
		}
		type spanCarrier interface {
			Span() (ast.Span, bool)
			Notes() []string
		}

		// If we have code + span, render with snippet
		if te, ok := e.(codedWithKey); ok && strings.TrimSpace(te.Code()) != "" {
			if sc, ok2 := e.(spanCarrier); ok2 {
				if sp, ok3 := sc.Span(); ok3 {
					file := a.file
					render := diag.Render(
						diag.Diagnostic{
							Domain:  te.Domain(),
							Key:     te.Key(),
							Code:    te.Code(),
							Level:   diag.LevelError,
							Message: te.Title(),
							Span:    diag.Span{Start: diag.Pos{Line: sp.Start.Line, Col: sp.Start.Col}, End: diag.Pos{Line: sp.End.Line, Col: sp.End.Col}},
							Notes:   sc.Notes(),
						},
						file,
						makeLineGetter(file),
					)
					term.Eprintf("%s", render)
					continue
				}
			}
			// fallback (no span)
			term.Eprintf("error[%s]: %s\n", te.Code(), te.Title())
			if h := lookupHelp(te.Domain(), te.Key()); strings.TrimSpace(h) != "" {
				term.Eprintf("help: %s\n", h)
			}
			continue
		}

		// final fallback
		term.Eprintf("error: %v\n", e)
	}

	if len(errs) > 0 || (a.werr && len(warns) > 0) {
		term.Eprintf("summary: %d error(s), %d warning(s)\n", len(errs), len(warns))
		return 1
	}

	// Emit C to gen/out
	base := strings.TrimSuffix(filepath.Base(a.file), filepath.Ext(a.file))
	outDir := filepath.Join("gen", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		term.Eprintf("mkdir %s: %v\n", outDir, err)
		return 1
	}
	cpath := filepath.Join(outDir, base+".c")
	csrc := cgen.EmitFile(merged, info)
	if err := os.WriteFile(cpath, []byte(csrc), 0o644); err != nil {
		term.Eprintf("write %s: %v\n", cpath, err)
		return 1
	}
	term.Eprintf("wrote %s\n", cpath)

	// Compile & link unless disabled
	if !a.noCC {
		outName := a.out
		if strings.TrimSpace(outName) == "" {
			outName = base
		}
		outBin := filepath.Join(outDir, outName)

		if err := cc.Compile(cc.Options{
			CSource:    cpath,
			Out:        outBin,
			RuntimeDir: a.runtimeDir, // empty => auto-detect runtime/c
			CCBin:      a.ccBin,      // empty => auto-pick per OS
			ExtraArgs:  a.ccArgs,     // pass-through flags
		}); err != nil {
			term.Eprintf("cc failed: %v\n", err)
			return 1
		}
		term.Eprintf("built %s\n", outBin)
	}

	term.Eprintf("summary: %d error(s), %d warning(s)\n", 0, len(warns))
	return 0
}

// tiny local helper so we don't import check in multiple files
func cgenCheckFileShim(f *ast.File) (*check.Info, []error, []check.Warning) {
	return check.CheckFile(f)
}

// --- helpers (moved here so we keep imports minimal) ---

// local minimal flag.ErrHelp replacement to avoid importing flag
var flagErrHelp = os.ErrInvalid

// guessErrFile tries to extract "load <path>:" prefix from ResolveAndParseWith loader errors.
// Falls back to the provided defaultFile if no path can be found.
func guessErrFile(errText, defaultFile string) string {
	var re = regexp.MustCompile(`(?i)\bload\s+(.+?):`)
	m := re.FindStringSubmatch(errText)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return defaultFile
}

// lookupHelp fetches the catalog help text for a (domain,key).
func lookupHelp(domain, key string) string {
	if info, ok := diag.LookupFull(domain, key); ok {
		return info.Entry.Help
	}
	return ""
}

// warnHelpFromCode resolves a warning code (DW...) back to a known key and returns help.
func warnHelpFromCode(code string) string {
	// Known warn keys we emit today.
	if info, ok := diag.LookupFull("warn", "unused_variable"); ok && info.Entry.ID == code {
		return info.Entry.Help
	}
	if info, ok := diag.LookupFull("warn", "unreachable_code"); ok && info.Entry.ID == code {
		return info.Entry.Help
	}
	if info, ok := diag.LookupFull("warn", "missing_explicit_return"); ok && info.Entry.ID == code {
		return info.Entry.Help
	}
	return ""
}

// makeLineGetter returns a closure that fetches 1-based source lines.
func makeLineGetter(file string) func(int) (string, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return func(int) (string, bool) { return "", false }
	}
	// Split without trimming trailing newline; we only need lines.
	raw := string(data)
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	return func(line int) (string, bool) {
		if line <= 0 || line > len(lines) {
			return "", false
		}
		return lines[line-1], true
	}
}
