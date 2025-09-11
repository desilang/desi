package main

import (
  "flag"
  "fmt"
  "os"
  "path/filepath"
  "strings"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/build"
  "github.com/desilang/desi/compiler/internal/cc"
  "github.com/desilang/desi/compiler/internal/check"
  cgen "github.com/desilang/desi/compiler/internal/codegen/c"
  "github.com/desilang/desi/compiler/internal/lexbridge"
  "github.com/desilang/desi/compiler/internal/parser"
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

  // diagnostics
  werr    bool // --Werror treats warnings as errors
  useDesi bool // --use-desi-lexer: route through lexbridge
  verbose bool // --verbose: pass to lexbridge
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
        return a, flag.ErrHelp
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
        return a, flag.ErrHelp
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
        return a, flag.ErrHelp
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
        return a, flag.ErrHelp
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
        return a, flag.ErrHelp
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
    }

    // first non-flag is entry file
    if !strings.HasPrefix(s, "-") && a.file == "" {
      a.file = s
      i++
      continue
    }
    if strings.HasPrefix(s, "-") {
      return a, flag.ErrHelp
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
    return a, flag.ErrHelp
  }
  return a, nil
}

func usageBuild() {
  term.Eprintln("usage: desic build [flags] <entry.desi>")
  term.Eprintln("\nGeneral:")
  term.Eprintln("  --use-desi-lexer         use the self-hosted Desi lexer via bridge")
  term.Eprintln("  --verbose                verbose bridge logging")
  term.Eprintln("  --Werror                 treat warnings as errors")
  term.Eprintln("\nC compile/link (enabled by default):")
  term.Eprintln("  --no-cc                  only emit C (skip compiling)")
  term.Eprintln("  --cc-bin=<cc>            choose compiler (clang/gcc/cl). Alias: --cc=<cc>")
  term.Eprintln("  --cc-arg=<flag>          pass through a flag to the C compiler (repeatable)")
  term.Eprintln("  --runtime-dir=<path>     override path to runtime/c (auto-detected otherwise)")
  term.Eprintln("  --out=<name>             output executable name (default: entry basename)")
}

func cmdBuild(args []string) int {
  a, err := parseBuildArgs(args)
  if err != nil {
    usageBuild()
    return 2
  }

  var (
    merged *ast.File
    perr   []error
  )

  // Choose lexer path
  if a.useDesi {
    loader := func(absPath string) (parser.TokenSource, error) {
      src, err := lexbridge.NewSourceFromFileOpts(absPath, false /*keepTmp*/, a.verbose /*verbose*/)
      if err != nil {
        return nil, err
      }
      ts, ok := src.(parser.TokenSource)
      if !ok {
        return nil, fmt.Errorf("lexbridge: internal type mismatch (value does not satisfy parser.TokenSource)")
      }
      return ts, nil
    }
    merged, perr = build.ResolveAndParseWith(a.file, loader)
  } else {
    merged, perr = build.ResolveAndParse(a.file)
  }

  if len(perr) > 0 {
    for _, e := range perr {
      term.Eprintf("error: %v\n", e)
    }
    term.Eprintf("summary: %d error(s), %d warning(s)\n", len(perr), 0)
    return 1
  }

  // Typecheck
  info, errs, warns := cgenCheckFileShim(merged)
  for _, w := range warns {
    term.Eprintf("warning: %s\n", w.String())
  }
  for _, e := range errs {
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
