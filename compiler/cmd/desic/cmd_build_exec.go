package main

import (
	"errors"
	"os"
	"path/filepath"
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

			// Typed diagnostics from loader.
			type codedWithKey interface {
				Code() string
				Title() string
				Domain() string
				Key() string
			}
			if te, ok := e.(codedWithKey); ok && strings.TrimSpace(te.Code()) != "" {
				codeUp := strings.ToUpper(strings.TrimSpace(te.Code()))

				// Classify warnings by code OR by Diagnostic.Level (using errors.As).
				isWarn := strings.HasPrefix(codeUp, "DW") || strings.HasPrefix(codeUp, "DMW")
				if !isWarn {
					var dd diag.Diagnostic
					if errors.As(e, &dd) && dd.Level == diag.LevelWarning {
						isWarn = true
					} else {
						var ddp *diag.Diagnostic
						if errors.As(e, &ddp) && ddp != nil && ddp.Level == diag.LevelWarning {
							isWarn = true
						}
					}
				}

				// Use full Error() text so details (module name, search paths, etc.) are shown.
				msg := e.Error()
				if isWarn {
					term.Eprintf("warning[%s]: %s\n", te.Code(), msg)
					if h := lookupHelp(te.Domain(), te.Key()); strings.TrimSpace(h) != "" {
						term.Eprintf("help: %s\n", h)
					}
					loaderWarns++
				} else {
					term.Eprintf("error[%s]: %s\n", te.Code(), msg)
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
