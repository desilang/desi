# Desi — tiny, fast, friendly (bootstrap)

This repo contains the bootstrap tooling for the Desi language:
- `desic` — command-line driver (demo flags during bootstrap)
- `desirepl` — minimal REPL (echo stub for now)
- `desifmt` — formatter stub
- Diagnostics catalog/renderer
- Token definitions (keywords, operators, punctuators, builtin types)



## Quick start

```bash
go build ./...
go run ./compiler/cmd/desic -version
go run ./compiler/cmd/desic -diag        # prints a demo diagnostic
go run ./compiler/cmd/desic -demo-tokens # prints sample token names
go run ./compiler/cmd/desic -demo-layout # prints layout events (NL/Indent/Dedent)
```

## Tree (high-level)

```
compiler/
  cmd/{desic,desirepl,desifmt}
  internal/
    diag/   # catalog + TTY renderer
    term/   # buffered stdout/stderr helpers
    token/  # token enums + tables
    lex/    # (bootstrap) layout pre-pass → NL/Indent/Dedent
docs/
  grammar.ebnf
  roadmap.md
  syntax.md
```
