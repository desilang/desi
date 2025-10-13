# Desi Compiler Roadmap (Revised, LLVM-first)

**Goals**
Pythonic surface • Rust-like safety/diagnostics • Elixir-style async • C/Rust/Java-class performance.
We ship **LLVM from day 1**, plus an interactive **REPL/IDLE**. Diagnostics are Rust-style with codes.

## Branching & releases
- Work happens on `revised` until “Revised MVP”. Then merge to `main`, tag `v0.1.0`.
- Conventional Commits; CI runs lint, unit tests, golden diagnostics, and formatter idempotence.
- Keep `legacy/` out of the new tree; no C backend.

---

## Milestones

### M0 — Bootstrap (DONE / in progress)
**Deliverables**
- `desic`, `desirepl`, `desifmt` stubs.
- Diagnostics core: JSON catalog (`docs/spec/codes.json`), TTY renderer, JSON output later.
- Repo layout reset under `compiler/` root.

**Acceptance**
- `desic -diag` prints Rust-style error and JSON form.
- `desirepl` starts and echoes lines.
- `desifmt` runs without errors on `.desi` files (stub behavior is OK).

---

### M1 — Tokens & Layout Lexer
**Scope**
- Implement Revised token set (keywords, builtins types, operators, punctuators).
- Indentation‐sensitive layout: `Indent`, `Dedent`, `NL`.
- Numeric/str/char literal scanning; f-string tokenization markers.
- Diagnostics: unterminated strings, invalid characters, mixed tabs/spaces (warning).

**Acceptance**
- `desic --dump-tokens file.desi` outputs a stable token stream.
- Golden tests for tricky indentation and f-strings.

---

### M2 — Parser & AST (Revised grammar)
**Scope**
- Implement full Revised EBNF:
  - def/async def, params (defaults, named args), decorators.
  - classes with implicit `self`, dunder rules, nested classes (visibility flags).
  - structs, enums, match (guards, value arms, `_` required generally).
  - pipelines `|>`, lambdas `=>`, comprehensions, for-in, using/RAII, defer.
  - multi-return annotations, one-line `if`/`while`, docstrings.
- AST nodes carry docstrings & decorator metadata.

**Acceptance**
- Parse `docs/syntax_tour.md` examples into AST without errors.
- Parser emits specific `DPE*` diagnostics and locations (golden tests).

---

### M3 — Resolver & Imports
**Scope**
- Module loader (project root + std); `import` / `from import`.
- Symbol tables, scopes, visibility:
  - top-level classes public by default; nested classes private unless `pub`.
  - **All dunders must be `pub`** (enforced).
- Detect duplicate/unused imports (warnings), import cycles, unknown modules.

**Acceptance**
- `desic check file.desi` reports undefined symbols, visibility errors with `DME*/DTE*`.
- Golden tests for import cycles and alias conflicts.

---

### M4 — Types & Overload Resolution (Phase 1)
**Scope**
- Concrete types: scalars, `list[T]`, `dict[K,V]`, `set[T]`, `tuple[...]`, `future[T]`, `none`.
- Function types, multi-return arity/types.
- Exact-match overloading by arity and parameter types.
- Basic inference for locals and call sites (no generics yet).
- Type rules for pipelines & multi-return feeding by arity.

**Acceptance**
- Overload selection deterministically picks the exact signature or errors with `DTE/OVL`.
- Chained comparisons type as `bool`. Pipelines type-check.

---

### M5 — Borrow Checker (Function-local, Phase 1)
**Scope**
- Param kinds: `T`(move), `ref T`(shared borrow), `inout T`(unique mutable borrow).
- Track states `{uninit, init, moved, borrowed(ro/rw)}`.
- Rule: **no `inout` borrow across `await`**; many `ref` or one `inout`, not both.

**Acceptance**
- Errors `DESI-BOR-*` with primary + secondary labels (where borrow started).
- Golden examples from the syntax tour compile/error as expected.

---

### M6 — HIR Lowering & Desugaring
**Scope**
- Lower: `using` → `defer __close__`, one-line `if/while` → blocks, comprehensions → loops, lambdas → function literals/closures, decorators → metadata/application.
- Normalize match guards and enforce `_` catch-all rule (where required).

**Acceptance**
- HIR dumper shows canonical form; unit tests on lowering passes.

---

### M7 — LLVM Codegen (Tier-0)
**Scope**
- IRBuilder module: functions, control flow, variables, calls, returns.
- Runtime bitcode: `__desi_print_*`, alloc/free, small string/array helpers.
- `desirepl` uses ORC JIT to evaluate expressions/defs.

**Acceptance**
- `desic build main.desi && ./main` prints expected hello world & small demos.
- `desirepl` can `print(1+2)` and define/run a simple function.

---

### M8 — Async & Futures
**Scope**
- Lower async funcs to state machines; `await`, `join`, `with_timeout`, `select`.
- Enforce borrow rules at suspension points.

**Acceptance**
- Demos from Revised (`greet`, `maybe_greet`, `race_two`) work.
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
- Enforce single-line/ multiline rules, spacing, and indentation.

**Acceptance**
- Golden format tests: input → expected output → reformat = no diff.

---

### M12 — Diagnostics Polish & Tooling
**Scope**
- Caret/underline renderer with color; JSON diagnostics (`--error-format=json`) for IDEs.
- Snapshot tests for error rendering.

**Acceptance**
- Matches Rust-like output style; IDE plugin PoC consumes JSON.

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
- **Documentation:** keep `docs/spec/grammar.ebnf` and `docs/syntax_tour.md` canonical and updated.
- **Style:** enforced by `desifmt` (no options).

