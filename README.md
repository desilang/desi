# Desi (Stage-0)

Desi is a **functional-first, performance-oriented** programming language with a Python-simple syntax, C/C++-class speed, and safety/concurrency ergonomics inspired by Rust and Elixir.

This repo contains the **Stage-0** toolchain:
- Compiler front-end in Go
- C code emitter backend (temporary, for bootstrap)
- Minimal C runtime (ARC, strings, vec)

## Quick start

```bash
# build the CLI
go build ./compiler/cmd/desic

# or run directly
go run ./compiler/cmd/desic --help
```

CI builds on Windows/macOS/Linux.

See `docs/` for the spec, roadmap, and contribution guide.
