# Desi (Stage-0 / Stage-1)

![Stage-0](https://img.shields.io/badge/Stage--0-stable-success)
![Stage-1](https://img.shields.io/badge/Stage--1-in_progress-orange)
![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-blue)

Desi is a **functional-first, performance-oriented** programming language with Python-simple syntax, C/C++-class speed, and safety/concurrency ergonomics inspired by Rust and Elixir.

This repo currently contains:

- **Stage-0**: Go front-end that emits portable C, plus a minimal C runtime (ARC, strings, vec, channels).
- **Stage-1**: A bridge that compiles and runs the **Desi-written lexer**, adapts its tokens, and feeds the existing Go parser. This is the first step toward full self-hosting.

## Quick start

### Requirements
- Go (compatible with your OS toolchain)
- A C compiler (we use `clang`)
- macOS/Linux/Windows supported

### Build the CLI
```bash
go build ./compiler/cmd/desic
# or run directly:
go run ./compiler/cmd/desic --help
```
```

### Parse with the legacy (Go) lexer

```bash
# macOS/Linux
go run ./compiler/cmd/desic parse examples/lex_demo.desi | head -n 20

# Windows (PowerShell)
go run .\compiler\cmd\desic parse .\examples\lex_demo.desi | Select-Object -First 20
```

### Use the Stage-1 Desi lexer (via bridge)

```bash
# Pretty tokens
go run ./compiler/cmd/desic lex-desi --format=pretty examples/lex_demo.desi | head -n 30

# Compare Go vs Desi token streams
go run ./compiler/cmd/desic lex-diff --limit=60 examples/lex_demo.desi

# Mapping coverage (Desi→Go kinds)
go run ./compiler/cmd/desic lex-map examples/lex_demo.desi
```

> Tip: add `--verbose` to surface native compiler output from the bridge if anything fails.

## Branches

* `main` — latest stable working tree (Stage-0 + Stage-1 tooling)
* `stage-0` — Stage-0 focus branch
* `stage-1` — Stage-1 work (bridge/adapter/CLI) — kept aligned with `main`

## Repository layout (high-level)

```
compiler/
  cmd/desic/                 # CLI
  internal/
    ast/ ...                 # AST + diagnostics
    check/ ...               # type checker
    codegen/c/emitter.go     # C emitter; println via printf; strcmp; concat
    lexbridge/               # Stage-1 bridge/adapter
      bridge.go              # BuildAndRunRaw
      desisource.go          # NewSourceFromFile(...) -> lexer.Source
      helpers.go             # escaping, NDJSON conversion, etc.
    lexer/ ...               # Go lexer (Stage-0)
    parser/ ...              # Go parser (Stage-0)
runtime/
  c/desi_std.[ch]            # ARC, str, vec, chan
examples/
  compiler/desi/lexer.desi   # The Desi lexer
  lex_demo.desi
docs/
  stages/
    stage-0.md
    stage-1.md               # Bridge & roadmap
gen/
  out/                       # emitted C + built bridge binary
  tmp/lexbridge/             # generated wrapper sources
```

## Project status

* ✅ **Stage-0**: stable, cross-platform; emits C and links the runtime.
* 🚧 **Stage-1**: Desi-lexer bridge and adapter working; mapping coverage and parser wiring in progress.
* 🛣️ **Next**: `parse --use-desi-lexer`, NDJSON at source, caching, Stage-1 end-to-end builds, and then parser self-hosting.

## Contributing

See `docs/stages/stage-0.md` and `docs/stages/stage-1.md` for scope, design notes, and checklists. PRs welcome—please include a brief description, reproduction steps (if a bug), and tests where applicable.
