# Desi (revised bootstrap)

A small, expression-first language with Python-style layout, fast builds, and clear tooling. This branch is a **revised bootstrap** focused on the lexer, diagnostics, and CLI scaffolding. Parsing and typing come next.

## Status

**Implemented in M0 (this branch):**
- Token definitions + helpers (`internal/token`)
- Lexer with layout (`Indent`/`Dedent`/`NL`) and greedy operator scan
- Numbers: `int` (dec/bin/oct/hex), `float` (with `e`/`E`), **hex floats** (`0x…p±…`)
- Underscore separators in numbers; `.5` and `2.` forms
- Strings: `"…"`, `"""…"""`, `f"…"`, with escape validation
- Non-fatal lexer errors surfaced via diagnostics (`internal/diag`)
- CLI demos: `-tokens`, `-diag`, `-demo-layout`
- **Indentation policy:** tabs-only (leading spaces produce a diagnostic)

## Build & test

```sh
go build ./...
go test ./...
````

## Try it

Dump tokens for a file:

```sh
go run ./compiler/cmd/desic -tokens examples/03_hex_float.desi
```

Emit a sample diagnostic:

```sh
go run ./compiler/cmd/desic -diag
```

See layout events:

```sh
go run ./compiler/cmd/desic -demo-layout
```

## Indentation policy

Desi enforces **tabs-only** for leading indentation. Lines starting with spaces produce `DLE0003` with a helpful fix-hint. Mid-line spaces for alignment are fine; only **leading** whitespace is checked.

## Where to read more

* [docs/syntax.md](docs/syntax.md) — human-friendly spec of what the lexer accepts.
* [docs/grammar.ebnf](docs/grammar.ebnf) — lexical EBNF for tokens/literals.
* [docs/roadmap.md](docs/roadmap.md) — high-level plan (parser/typers next).
* [docs/syntax.md](docs/syntax.md)#Diagnostics — diagnostic codes surfaced by the lexer.

## License

MIT — see [LICENSE](LICENSE).
