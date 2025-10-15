# Desi Compiler Roadmap (Revised, LLVM-first)

**Goals**
Pythonic surface • Rust-like safety/diagnostics • Elixir-style async • C/Rust/Java-class performance.
We ship **LLVM from day 1**, plus an interactive **REPL**. Diagnostics are Rust-style with codes.

## Branching & releases
- Work happens on `revised` until “Revised MVP”. Then merge to `main`, tag `v0.1.0`.
- Conventional Commits; CI runs lint, unit tests, golden diagnostics, and formatter idempotence.
- Keep `legacy/` out of the new tree; no C backend.

---

## Milestones

### M0 — Bootstrap (✅ DONE)
**What shipped in M0 (revised & expanded):**
- **Token layer**: enums, keyword/builtin-type/operator tables, `Token.String()`, `TokenCategory()`.
- **Lexer**: indentation-sensitive layout (`NL`, `Indent`, `Dedent`), identifiers/keywords, greedy operators
  (`**`, `**=`, `^`, `^=`, `|>`, `>=`, `==`, etc.), punctuators.
- **Numbers**: dec/bin/oct/hex integers; decimal floats (with `e`/`E`); **hex floats** `0x…p±…`;
  `_` digit separators; `.5` and `2.` forms; robust error recovery on invalid literals.
- **Strings**: `"…"`, `"""…"""`, `f"…"` (reserved; no interpolation yet); validated escapes
  (`\\ \" \n \r \t \0 \xNN \uXXXX \UXXXXXXXX \{ \}`); correct long-string closing.
- **Diagnostics**: JSON catalog + builder; TTY renderer; non-fatal lexer errors surfaced; CLI caps excessive errors.
- **Indentation policy**: **tabs-only** (spaces at BOL produce `DLE0003`, scanning continues).
- **CLI**: `desic -tokens`, `-diag`, `-demo-layout`, `-version`.
- **Tests**: greedy operator cases; indentation (tabs-only) assertions; scanner tests green.
- **Docs**: `docs/guides/m0-basics.md` (numbers, strings, layout, operators, diagnostics);
  updated lexical **EBNF** (`docs/grammar.ebnf`) incl. parenthesized multi-line `from … import (…)`.

**Acceptance (met):**
- `go build ./...` and `go test ./...` pass.
- `desic -diag` prints Rust-style error and JSON form.
- `desic -tokens <file.desi>` prints a stable token stream, then diagnostics (capped).
- `desic -demo-layout` shows layout events for a sample file.

---

### M1 — Parser & AST (Phase 1, ✅ DONE)
**Scope shipped**
- **AST**: `Module`, `FuncDecl` (with `Async` flag), `Block`, `LetStmt`, `ReturnStmt`, `ExprStmt`;
  expressions: literals/idents, unary (`- ! not await`), power `**` (right-assoc),
  `* / %`, `+ -`, `^`, `< <= > >=`, `== !=`, pipeline `|>`, logical `and/or`.
- **Postfix**: greedy `call()/index[]/field .` in any order.
- **Types (minimal)**: `TypeName` as plain/dotted identifiers (no generics yet).
- **Diagnostics (parser)**: `DPE0001` unexpected, `DPE0002` expected, `DPE0003` unclosed delimiter;
  async-specific: `DPE1001` (async only before `def`), `DPE1002` (async not allowed before `let`).
- **CLI**: `desic -ast <file>` pretty-prints AST, then renders diagnostics via `codes.json`.
- **Behavioral niceties**: comment/blank lines before first block indent; EOF acts like newline.
- **Tests**: golden AST for power+pipeline, postfix chains, async def, EOF-as-NL, comment-before-indent,
  and DPE0003 coverage.

**Acceptance**
- `go build ./...` and `go test ./...` pass.
- `go run ./compiler/cmd/desic -ast examples/07_ast_smoke.desi` shows the expected AST.
- Existing examples parse; files with non-M1 ops (`|`, `**=`) show targeted parser errors.

**Deferred from M1 (planned later)**
- Bitwise `|` tier; augmented assignment `**=` (keep in lexer; parse later with assignment rules).
- Lambdas (sync & async), comprehensions, control flow, classes, decorators, imports, etc.

---

### M2 — Parser & AST (Revised grammar, Phase 2)
**Scope**
- Extend to the revised EBNF:
  - Decorators, classes (implicit `self`), enums/structs, `match`, comprehensions,
    `for in`, `using`/`defer`, one-line conditionals/loops, docstrings.
  - **Bitwise `|` tier** and **augmented assignments** (including `**=`).
  - **Lambdas (`=>`)** – sync only in this phase.
- AST nodes carry docstrings & decorator metadata.

**Acceptance**
- Parse `docs/syntax_tour.md` examples into AST without errors.
- Parser emits specific `DPE*` diagnostics (golden tests).

---

### M8 — Async & Futures (adds async lambda)
**Scope**
- Lower async funcs to state machines; `await`, `join`, `with_timeout`, `select`.
- Enforce borrow rules at suspension points.
- **Add `async lambda` expression** (expression-only body):
  `async lambda x: await f(x)` (types: `AsyncFunc[A, B]` ≈ `Func[A, future[B]]`).

**Acceptance**
- Demos from Revised run; async lambda covered by tests.
- Reject `inout` across await with clear diagnostics.

---

### M9 — FFI v1 (C ABI)
**Scope**
- **A)** Manifest-driven linking (`desi.toml`).
- **B)** `@extern("C", link="...")` decorators.
- `cptr[T]`, `usize/isize`, null checks, minimal `unsafe` blocks.
- Examples: `libm` (`sin/cos`), `sqlite3`, minimal OpenSSL SHA256.

**Acceptance**
- `unit_circle` & sqlite demos run.
- Type mismatches produce `DESI-FFI-*` with helpful help text.

---

### M10 — Collections & Prelude v1
**Scope**
- std prelude: `print`, `len`, `bool`, `str`, `map`, `filter`, `range`.
- Built-in collection ops (`push`, `set`, membership `in`).
- Comprehensions compiled efficiently.

**Acceptance**
- Collections & comprehensions examples pass and are fast (microbench).

---

### M11 — Formatter v1 (AST pretty-printer)
**Scope**
- Idempotent; no options initially; respects docstrings/decorators/import grouping.
- Enforce single-line/multiline rules, spacing, and indentation.

**Acceptance**
- Golden format tests: input → expected output → reformat = no diff.

---

### M12 — Diagnostics Polish & Tooling
**Scope**
- Caret/underline renderer with color; JSON diagnostics (`--error-format=json`) for IDEs.
- Snapshot tests for error rendering.
- **Developer ergonomics:** add a simple `Makefile` with handy targets:
  - `make build`, `make test`
  - `make tokens FILE=examples/…`
  - `make demo-layout`
  - later: `make fmt`, `make vet`, `make lint`
- (Optional) Hook `go vet` / `staticcheck` / basic CI workflows.

**Acceptance**
- Matches Rust-like output style; IDE plugin PoC consumes JSON.
- `make test` green; helper targets work locally and in CI.

---

### M13 — Packaging & Std Growth
**Scope**
- `desi init`, `desi build`, `desi run`, `desi test`.
- Expand std modules (`string`, `task`, `fs`, `time`), dataclass decorator, etc.

**Acceptance**
- Example projects build/run; documentation synced.

---

## Cross-cutting practices
- **Testing:** unit tests per package, golden tests for diagnostics & formatter, integration smoke tests for codegen.
- **Performance:** microbenchmarks for tight loops, pipelines, comprehensions; keep codegen O2-friendly.
- **Documentation:** keep `docs/grammar.ebnf` and `docs/guides/*` canonical and updated.
- **Style:** enforced by `desifmt` (no options).
