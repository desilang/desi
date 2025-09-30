package parsebridge

import (
  "fmt"
  "os"
  "path/filepath"
  "runtime"
  "strings"

  "github.com/desilang/desi/compiler/internal/term"

  "github.com/desilang/desi/compiler/internal/check"
  cgen "github.com/desilang/desi/compiler/internal/codegen/c"
)

// ----- public entrypoint -----

// BuildAndRunJSON compiles (and caches) a tiny wrapper that calls
// compiler.desi.parser::parse_to_json(path) and returns its stdout (AST JSON).
// Requires a dev parser at examples/compiler/desi/parser.desi.
// We also mirror examples/compiler/desi/lexer.desi so the parser can import it.
func BuildAndRunJSON(entryFile string, keepTmp, verbose bool) ([]byte, error) {
  entryAbs, err := filepath.Abs(entryFile)
  if err != nil {
    return nil, fmt.Errorf("abs(%s): %v", entryFile, err)
  }

  // dev sources
  repoRelParser := filepath.Join("examples", "compiler", "desi", "parser.desi")
  devParserSrc, err := os.ReadFile(repoRelParser)
  if err != nil {
    return nil, ParserSourcesMissingError{Path: repoRelParser}
  }
  repoRelLexer := filepath.Join("examples", "compiler", "desi", "lexer.desi")
  devLexerSrc, _ := os.ReadFile(repoRelLexer)

  // runtime bits
  rtDir := filepath.Join("runtime", "c")
  rtC := filepath.Join(rtDir, "desi_std.c")
  rtH := filepath.Join(rtDir, "desi_std.h")
  rtCSrc, _ := os.ReadFile(rtC)
  rtHSrc, _ := os.ReadFile(rtH)

  // wrapper source
  wrapper := buildWrapper(entryAbs)

  // cache key
  sig := sha256Sum(
    "parsebridge-v3",
    runtime.GOOS, runtime.GOARCH,
    string(wrapper),
    string(devParserSrc),
    string(devLexerSrc),
    string(rtCSrc), string(rtHSrc),
  )
  cacheRoot := filepath.Join("gen", "tmp", "parsebridge", "cache", sig)
  binPath := filepath.Join(cacheRoot, "parsebridge_run")
  if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binPath), ".exe") {
    binPath += ".exe"
  }

  // fast path
  if fileExists(binPath) {
    return runAndClean(binPath, cacheRoot, verbose)
  }

  // work dir
  if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
    return nil, fmt.Errorf("mkdir %s: %w", cacheRoot, err)
  }

  // mirror dev parser/lexer
  if err := mirrorDevParserInto(cacheRoot, devParserSrc); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    return nil, err
  }
  if len(devLexerSrc) > 0 {
    if err := mirrorDevLexerInto(cacheRoot, devLexerSrc); err != nil {
      if !keepTmp {
        _ = os.RemoveAll(cacheRoot)
      }
      return nil, err
    }
  }

  // write wrapper
  wrapperPath := filepath.Join(cacheRoot, "main.desi")
  if err := os.WriteFile(wrapperPath, wrapper, 0o644); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    return nil, fmt.Errorf("write wrapper: %w", err)
  }

  // parse+typecheck wrapper locally
  merged, perr := resolveAndParseLocal(cacheRoot, wrapperPath)
  if len(perr) > 0 {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    var b strings.Builder
    for _, e := range perr {
      term.Bprintf(&b, "error: %v\n", e)
    }
    return nil, fmt.Errorf("resolve/parse wrapper failed:\n%s", b.String())
  }
  info, errs, _ := check.CheckFile(merged)
  if len(errs) > 0 {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    var b strings.Builder
    for _, e := range errs {
      term.Bprintf(&b, "error: %v\n", e)
    }
    return nil, fmt.Errorf("typecheck wrapper failed:\n%s", b.String())
  }

  // emit C and compile
  cpath := filepath.Join(cacheRoot, "main.c")
  csrc := cgen.EmitFile(merged, info)
  if err := os.WriteFile(cpath, []byte(csrc), 0o644); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    return nil, fmt.Errorf("write %s: %w", cpath, err)
  }
  if err := compileBin(cpath, rtDir, binPath, verbose); err != nil {
    if !keepTmp {
      _ = os.RemoveAll(cacheRoot)
    }
    if verbose {
      return nil, fmt.Errorf("cc failed: %w", err)
    }
    return nil, fmt.Errorf("cc failed; re-run with --bridge-verbose to see compiler output")
  }

  // run bridge program and return cleaned JSON
  return runAndClean(binPath, cacheRoot, verbose)
}
