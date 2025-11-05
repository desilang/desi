# Desi (revised bootstrap)

A small, expression-first language with Python-style layout, Rust-like safety, and clear tooling.
This branch is the **revised bootstrap**, now well beyond lexer: we have parser, type checker,
borrow rules (M6), imports/resolution (M5), HIR + lowering, and an LLVM backend under test.
Diagnostics are single-sourced from a catalog.

## Status (high-level)

**Core implemented**

- **Lexer** with layout (`Indent`/`Dedent`/`NL`) and greedy operator scan
  - Numbers: `int` (dec/bin/oct/hex), `float` (`e`/`E`), **hex floats** (`0x…p±…`)
  - Underscore separators; `.5` / `2.` accepted
  - Strings: `"…"`, `"""…"""`, `f"…"`, escape validation
  - Tabs-only indentation policy (leading spaces produce a diagnostic)
- **Parser** for classes/structs/enums, flow statements, `match`, `using`/`defer`,
  lambdas (incl. parenthesized), slices/steps, and comprehensions
- **Type checker** with overloads, pipelines, multi-return, comprehensions
- **Borrow checking (M6)** including caller/callee rules & last-use paths
- **Imports/Resolver (M5)** with `__mod.desi` module entry support and cycle handling
- **HIR + lowering** (incl. async lowering stubs) and LLVM module emission (tests present)
- **Diagnostics**: unified renderer pulling titles/help/suggestions from a single **codes.json**

**In progress / staged**

- **M7+**: more async + runtime shaping
- **M8** groundwork visible in examples (async basic), more rules to come

## Build & Test

```sh
go build ./...
go test ./...
```

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

Type & borrow checking for a file:

```sh
go run ./compiler/cmd/desic -I "examples:compiler/lib" -check examples/12_m4_types_overload.desi
```

Lower/HIR → LLVM IR preview (example uses async sample):

```sh
go run ./compiler/cmd/desic emit-ir examples/15_m8_async_basic.desi
```

## Indentation policy

Desi enforces **tabs-only** for leading indentation. Lines starting with spaces produce a diagnostic with a helpful fix-hint. Mid-line spaces for alignment are fine; only **leading** whitespace is checked.

## Diagnostics

Desi’s diagnostics are **single-sourced** from a catalog:

* Titles, help text, and suggestions live in `compiler/internal/diag/codes.json`
* The renderer (`compiler/internal/diag/render_tty.go`) prefers catalog values
* New codes must be added to the catalog (tests enforce this)

Read the full guide: **[docs/dev/diagnostics-catalog.md](docs/dev/diagnostics-catalog.md)**

## Where to read more

* **Syntax overview:** [docs/syntax.md](docs/syntax.md)
* **Grammar (EBNF):** [docs/grammar.ebnf](docs/grammar.ebnf)
* **Roadmap:** [docs/roadmap.md](docs/roadmap.md)
* **Imports & resolver:** [docs/guides/m5-imports.md](docs/guides/m5-imports.md)
* **Borrow rules (M6):** [docs/guides/m6-borrow.md](docs/guides/m6-borrow.md)
* **LLVM IR optimization notes:** [docs/dev/llvm-ir-optimization.md](docs/dev/llvm-ir-optimization.md)
* **Memory management notes:** [docs/dev/memory-management.md](docs/dev/memory-management.md)

## License

MIT — see [LICENSE](LICENSE).
