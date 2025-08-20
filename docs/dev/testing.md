# Testing

We rely on a mix of **unit tests** and **golden tests** to keep the compiler stable during rapid iteration.

## Running tests
```bash
go test ./...
```

## Unit tests

* Live under `tests/<pkg>/*_test.go` or alongside packages when tight coupling helps.
* Cover token scanning cases, parser corner cases, and error diagnostics.

## Golden tests

* Compare current output (tokens, AST, emitted C) against a stored “golden” file.
* Keep golden files small and deterministic.
* Update goldens only via deliberate `-update` flags in test helpers (to avoid accidental churn).

## What to test first (Stage-0)

* **Lexer**: identifiers, keywords, numbers (dec/hex/bin), strings (escapes), indentation, `:=` vs `=`.
* **Parser**: precedence/associativity, `match` arms, function signatures, struct/enum forms.
* **Diagnostics**: positions for common errors (unexpected token, bad indent, missing newline).
* **Runtime smoke**: ARC retain/release counts under simple programs.

## CI

* GitHub Actions runs `go build` and `go test` on macOS, Windows, Linux.
* Keep tests OS-agnostic; file path tests should normalize separators.
