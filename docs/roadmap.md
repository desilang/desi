# Desi Compiler Roadmap (Revised, LLVM-first)

**Goals**
Pythonic surface • Rust-like safety/diagnostics • Elixir-style async/futures • C/Rust/Java-class performance.
We ship **LLVM from day 1**, plus an interactive **REPL**. Diagnostics are Rust-style with codes.

## Branching & releases

* Work happens on `revised` until “Revised MVP”. Then merge to `main`, tag `v0.1.0`.
* Conventional Commits; CI runs lint, unit tests, golden diagnostics, and formatter idempotence.
* Keep `legacy/` out of the new tree; **no C backend**.

---

## Milestones

### M0 — Bootstrap (✅ DONE)

**What shipped**

* **Token layer**: enums, keyword/builtin-type/operator tables, `Token.String()`, `TokenCategory()`.
* **Lexer**: indentation-sensitive layout (`NL`, `Indent`, `Dedent`), identifiers/keywords, greedy operators
  (`**`, `**=`, `^`, `^=`, `|>`, `>=`, `==`, etc.), punctuators.
* **Numbers**: dec/bin/oct/hex integers; decimal floats (with `e`/`E`); **hex floats** `0x…p±…`; `_` digit separators; `.5` & `2.` forms; robust recovery on invalid literals.
* **Strings**: `"…"`, `"""…"""`, `f"…"` (reserved; no interpolation yet); validated escapes; correct long-string closing.
* **Diagnostics**: JSON catalog + builder; TTY renderer; non-fatal lexer errors surfaced; CLI caps excessive errors.
* **Indentation policy**: **tabs-only** (spaces at BOL → `DLE0003`, scanning continues).
* **CLI**: `desic -tokens`, `-diag`, `-demo-layout`, `-version`.
* **Tests**: greedy operator cases; indentation (tabs-only) assertions; scanner tests green.
* **Docs**: updated lexical **EBNF** and quick guides.

**Acceptance**

* `go build ./...` and `go test ./...` pass.
* `desic -diag` prints Rust-style error and JSON form.
* `desic -tokens <file.desi>` prints a stable token stream (then diagnostics, capped).
* `desic -demo-layout` shows layout events for a sample file.

---

### M1 — Parser & AST (Phase 1, ✅ DONE)

**Scope**

* **AST**: `Module`, `FuncDecl` (with `Async` flag), `Block`, `LetStmt`, `ReturnStmt`, `ExprStmt`.
* **Expressions**: literals/idents, unary (`- ! not await`), power `**` (right-assoc), `* / %`, `+ -`, `^`, comparisons `< <= > >=`, equality `== !=`, pipeline `|>`, logical `and/or`.
* **Postfix**: greedy `call()/index[]/field .` in any order.
* **Types (minimal)**: `TypeName` as plain/dotted identifiers (no generics yet).
* **Diagnostics (parser)**: `DPE0001` unexpected, `DPE0002` expected, `DPE0003` unclosed delimiter; async-specific: `DPE1001` (async only before `def`), `DPE1002` (async not allowed before `let`).
* **CLI**: `desic -ast <file>` pretty-prints AST via codes catalog.

**Acceptance**

* `go build ./...`, `go test ./...` pass.
* `-ast` shows the expected tree for `examples/07_ast_smoke.desi`.
* Files using out-of-phase ops (e.g., bitwise `|`, `**=`) produce targeted parser errors.

---

### M2 — Parser & AST (Revised grammar, Phase 2, ✅ DONE)

**Scope**

* Extend to revised EBNF elements (parse-only): decorators, docstrings, `for in`, `using`/`defer`,
  one-line conditionals/loops, `match` (surface-only), and groundwork for classes/enums/structs/comprehensions.
* Bring in bitwise-`|` tier and augmented assignments (including `**=`) at the expression level.
* AST nodes carry docstrings & decorator metadata.

**Acceptance**

* Examples from the syntax tour parse into AST without errors.
* Parser emits `DPE*` diagnostics with stable text/locations.

---

### M3 — Surface Constructs (parse-only, ✅ DONE)

> Implement high-value language constructs **parse-only** with AST + printer and strong diagnostics.

**M3A — Classes** (✅)
* Classes with optional bases, nested classes.
* Decorators on classes/methods; docstring captured at top of class body.
* Visibility: **top-level classes public by default**, **nested classes private by default** (unless `pub`).
* Fields parse as `["pub"] Ident ":" Type`.
* Printer renders decorators, `pub`, bases, docstring, fields, nested classes, methods.

**M3B — Structs & Enums** (✅)
* **Structs**: top-level structs private by default; fields can be `pub`.
* **Enums**: top-level enums private by default; variants `Name : TypeName | none`.
* Printer support.

**M3C/D — Lambdas & Comprehensions** (✅)
* Lambdas: single-param + parenthesized/typed params.
* Comprehensions: list/dict/set; multi-`for` chains with optional per-clause `if`.
* Crisp diagnostics for missing parts.

**Acceptance**
* `go build ./...` and `go test ./...` pass; `-ast` shows sane trees for new examples.
* No `<?>` placeholders in printed AST.

---

### M4 — Types & Overload Resolution (Phase 1, ✅ DONE)

**Scope**

* **Concrete types**: `int`, `float`, `bool`, `str`, `none`; constructed `list[T]`, `set[T]`, `dict[K,V]`, `tuple[...]`, `future[T]` (stub ok).
* **Functions & multi-return**: `func(...) -> T`; grouped assignment enforces **width** & element-wise type checks.
* **Exact-match overloading** by **arity + parameter types**.
* **Basic inference** for literals, lets/assign/aug-assign, comparisons/logicals.
* **Pipeline `|>`** typing: treat `a |> f(b, c)` as `f(a, b, c)`.
* **Comprehensions**: element/key/value types propagate into `list/set/dict`.
* **Lambdas**: typed params supported (tests require typed params in this phase).
* **Match (parse-only)**: require all arm results to have the **same** type.

**Acceptance**

* Deterministic overload selection or precise `DTE*` errors.
* Pipelines type-check; grouped-assign width/type rules enforced.
* Tests cover arithmetic, overloads (ok/no-match/ambiguous/arity), pipeline, comprehensions (ok/negative), typed lambdas, grouped assignment, and match arms typing.

---

### M5 — Resolver & Imports (Phase 1–4, ✅ DONE)

**Phase-1 (✅)**: Syntax, loader with `__mod.desi`, multi-root search, graph/cycles, prelude injection, basic diags.
**Phase-2 (✅)**: Real cross-module function signatures via resolver **Exports**; only **`pub def` + fully typed** are exported. Checker consumes exact overloads for `from … import …`. `DME0003` on missing export. **Unused-import lints** (`DMW0004/5`) implemented.
**Phase-3 (✅ — Module-qualified calls):** `import math; math.add(…)` resolves using `math` exports; non-export → `DME0003`. Qualifier use counts for unused-import lint.
**Phase-4 (✅ — Ergonomics v1):** implicit `str` on `+` with `str` operands; **f-strings (stage 1)** recognized as `str` literals; tuple unpacking multi-LHS; slice steps parsed; **string slices type to `str`**.

---

### M6 — Borrow Checker (Function-local, ✅ DONE)

**Scope**

* Param kinds: `T` (move), `ref T` (shared), `inout T` (unique mutable).
* Track states `{uninit, init, moved, borrowed(ro/rw)}`.
* Rule: **no `inout` borrow across `await`**; many `ref` or one `inout`, not both.
* Ergonomics/diags: aliasing diagnostics carry **secondary labels** to the conflicting argument/site; primitives (`int/float/bool/str`) are **copy** and don’t trigger DBR on by-value pass; `ref` requires lvalue.

**Acceptance**

* `DBR0001–DBR0005` cataloged; golden borrow/move tests green.

---

### M7 — HIR Lowering & LLVM Codegen (Tier-0, ✅ DONE)

**What shipped**

* **HIR skeleton** with **named temporaries** and a pretty-printer.
* **Deterministic drops (RAII)**; `using arena:` → single epilogue destroy; `Drop` → `DecRef` for rc/arc.
* **CLI emitter**: `desic emit-ir <file.desi>`: AST→HIR→**textual LLVM IR**.
  * Built-in `print("…")` lowers to `puts` with private `@.str.N` globals.
  * Conservative `llvm.lifetime.start/end` for locals.
  * Demand-driven declarations for `__rc_dec` / `__arena_destroy`.

**Acceptance**
* Tests for drops/arenas/async stubs; `examples/14_m7_main.desi` emits valid `.ll`.

---

### M8 — Async & Futures (✅ DONE)

**What shipped**

* **Async** lowering to wrapper + `poll(%frame)`; `frame.set/get` around each `await`.
* Enforce **DBR0001** for `inout` across `await`.
* **Async lambdas** via desugaring to hidden async functions.
* Backend polish: pointer-return wrappers, typed headers, correct lifetime placement.

**Acceptance**
* Comprehensive lowering/backend tests; examples compile and emit expected IR.

---

### M9 — FFI v1 (C ABI, ✅ DONE)

**What shipped**

* Types: `usize/isize` (→ `i64`), `cptr[T]` (→ `ptr`); null comparable.
* **unsafe:** blocks for extern calls; **DFI0003** in safe context.
* `@extern("C"[,"lib"])` on `pub def`; resolver carries extern metadata; backend emits **one** `declare` per referenced symbol.
* Re-exports supported through package mod; library stubs under `compiler/lib/*`.

**Acceptance**
* Externs require `unsafe:`; single `declare` emitted; examples parse/check/emit IR.

---

### M10 — Collections & Prelude v1 (✅ DONE)

**What shipped**

* Prelude injects `print/len/str/bool` (shadow → **DPL0001**).
* `len("…")->int` (others → **DCO0001**); `"a" in "abc"->bool` (unsupported → **DCO0002**).
* Compile-only stubs: `list_push/set_add/dict_set`.
* `range()` surface; list comps lower to tight loops using `list_push`.
* `map/filter` desugar before type-check **and** before `emit-ir`.
* **Typed LLVM headers** for user functions; lifetime placement polished.

**Acceptance**
* Examples and tests cover all paths; repo stays green.

---

### M11 — Formatter (✅ DONE)

> **Status:** Shipped as a **token-rewrite formatter** (`internal/format`), plus a small polish release (**M11.1**) for EOL comments & docstrings. An AST pretty-printer is deferred to **M11.2** (see below).

**What shipped (Formatter v1)**
* Deterministic **token rewriter**:
  * Rewrites **whitespace/trivia only**; does **not** change tokens.
  * **Tabs-only** at BOL; **no trailing spaces**; single trailing newline.
  * Spacing rules: around binary ops/assign/pipe; tight field `.`; no space before call/index; `:` spaced except inside slices.
* **String literal reconstruction** for empty-lexeme string tokens; long strings bounded so they **never duplicate**.
* **CLI** `desifmt`:
  * Reads stdin on `-` (uses `os.Stdin`/`os.Stdout`), prints to stdout by default; `-w` writes in place; `-l` lists changed files; `-q` quiet.
  * Exit codes: `0` success, `1` on parse diags, `2` for I/O/arg errors.
* **Tests**
  * Golden Phase-1/2 and idempotence tests; smoke tests; repo-wide smoke on examples.

**M11.1 — Comments & Docstrings polish (✅ DONE)**
* Preserve **end-of-line `#` comments** that trail real code (ignore `#{` set-literal openers; never move comment-only lines).
* Hardened **multi-line docstring** reconstruction (no duplication, no spill across tokens).
* Added Phase-3 goldens and idempotence; improved test harness output.

**M11.2 — Formatter v2 (AST pretty-printer, Deferred)**
* **Goal:** full AST-backed pretty-printer (import grouping, line-wrapping, break rules).
* **Why deferred:** no comment/trivia model attached to AST; grammar edges (tuples/lists/comps) still settling; high churn risk.
* **Prereqs:**
  - Comment/trivia attachment plan (design + implementation).
  - Finalize wrap/break rules (width, hanging indent, call-arg wrapping).
  - Tuple/list/comprehension grammar cleanups (ambiguity/precedence).
* **Queued (not part of M11.2 itself):**
  - Tuple syntax rework & tuple-return pattern disambiguation.
  - Bare list literal vs. comprehension parser sharp edges.
  - Block comment model (if we ever add it).
  - F-string interpolation semantics beyond stage-1 (currently plain strings).
  - Any AST/IR redesign — kept out of formatter milestones.
* **Scheduling:** revisit **after M12** once diagnostics and parser/style are stable; no downstream blockers.

---

### M12 — Diagnostics Polish & Tooling (✅ DONE)

**What shipped**

* **TTY renderer v1** (`internal/diag/render`):
  * Rust-style header: `file:line:col: error[CODE] domain: title`.
  * **Caret/underline** with tab-aware placement (tabs expand to 8; original tabs preserved in source line).
  * **Multi-label** support: primary vs secondary labels; optional trailing label text; Unicode rune counting for alignment.
  * **Color policy** `--color=auto|always|never`; **auto enables on TTY**, and color **disables** when `NO_COLOR` is set or output is not a TTY.
* **JSON diagnostics** (`--error-format=json`):
  * Stable schema (`code/domain/title/message/severity/primary/labels/notes/help`), no HTML escaping (keeps `"<stdin>"` readable).
  * **Atomic emission**: CLIs buffer all diagnostics and emit **a single JSON array to stderr** per run.
* **CLI plumbing**:
  * Both `desic` and `desifmt` honor `--error-format` and `--color` globally.
  * Subcommand UX: `desic check --error-format=json …` works (render flags stripped before `parseCheckArgs`).
  * No `/dev/stdin` tricks; uses `os.Stdin`/`os.Stdout`/`os.Stderr`.
* **Snapshot tests**:
  * Deterministic TTY/JSON snapshots under `compiler/internal/diag/testdata/*` with CRLF normalization and fixed width.
  * Tests use external package (`diag_test`) to avoid import cycles, cover parser-generated errors and renderer alignment.
* **Makefile** (top level):
  * `build`, `test`, `tokens`, `demo-layout`, `fmt`, `repl` targets; `fmt` guards empty file lists via `git ls-files`.

**Acceptance**

* `go build ./... && go test ./...` **green**.
* `desic --error-format=json …` and `desifmt --error-format=json …` emit **one JSON array** to **stderr**; human mode unchanged with color policy.
* Snapshot tests for **TTY** and **JSON** pass across platforms.
* Makefile targets work locally (and suitable for CI).

---

### M13 — Project Manifest & Packaging + Call/Param Polish (🚧 In progress)

**Scope**

* **Project manifest**: `desi.mod` (DML/INI-like), parser/validator in `internal/project` with `DPM*` diags; `FindRoot` and `Load`.
* **Manifest-aware CLIs**: `desic init|run|build|test`
  * `init NAME [-p PATH] [-v VERSION] [-e EDITION] [--force]`: scaffolds `desi.mod`, `src/main.desi`, `tests/`.
  * `run`: resolve `entry`/`roots` from manifest (unless overridden); type-check.
  * `build`: emit textual LLVM IR to `<out_dir>/<package>.ll`; `--verify-llvm` optionally runs `llvm-as` if available.
  * `test`: placeholder → `go test ./...` (Desi test runner later).
  * **Precedence**: **CLI > env > manifest > defaults**; diagnostics defaults from `[diagnostics]` are seeded early and still overridden by flags.
* **Call/Param polish**:
  * **Named arguments** (no splats). After the first named arg, all following must be named. Diags: `DCA0001` unknown name, `DCA0002` duplicate, `DCA0003` positional after named.
  * **Positional varargs `*args`** (safe subset): final param `*p:T` treated as `list[T]`; call-site `*Expr` must type to `list[T]`. Diags: `DCA0010` wrong element type, `DCA0011` `inout` via varargs forbidden.

**Acceptance**

* `desic init demo && cd demo && desic run` succeeds; `desic build` writes IR file.
* Renderer flags (`--error-format`, `--color`) honored per precedence; JSON mode emits a single array to **stderr**.
* Named args canonicalize correctly; varargs collect positionals and starred lists with precise diags.

---

### M14 — Defaults, Display, Import-Closure IR, and Stdlib Growth (🆕 Planned)

**Goals**
Deliver Python-like ergonomics without dynamic typing: default parameters, a lightweight `Display` trait for stringification, robust `print` API on top, import-closure IR emission, and a first useful wave of stdlib modules written in Desi.

**Language & Checker**
* **Parameter defaults**:
  * Grammar: `Param := [mode] Ident ":" Type [ "=" ConstExpr ]`.
  * Rules: defaults allowed for `move` and `ref`; **not** for `inout`. Defaults must be **compile-time const** (literals, enum variants, `len("…")` and similar constfolded intrinsics).
  * Overload selection unchanged; after selecting the overload, the checker fills omitted actuals from defaults. Arity errors/diags updated accordingly.
  * Diagnostics: `DDF0001` default not const; `DDF0002` default on `inout`; `DDF0003` missing required arg; `DDF0004` conflicting defaults across overloads (if applicable).
* **`Display` trait (or `Show`)**:
  * Provide a prelude trait with `to_str(self) -> str`.
  * Built-in impls: `int`, `float`, `bool`, `str`, potentially `char`.
  * Enables typed stringification without `any`.
* **`print` on top of E/F + defaults**:
  * Public API: `def print(*args: list[Display], sep: str = " ", end: str = "\n") -> int`.
  * Internals: `print_core(args: list[str], sep: str, end: str) -> int` + wrapper that maps `Display`→`str`.
  * Examples updated accordingly.

**Backend & Build**
* **Import-closure IR emission**:
  * `desic build` gathers the entry module **and all imported, reachable `pub` defs** from `roots` and emits a **single** `.ll` module.
  * Ensure deduped declarations/definitions and stable symbol names. Extend `types_lower` tests and add `emit_smoke` coverage for cross-module calls.
* **LLVM verification (polish)**:
  * Keep `--verify-llvm` flag; add CI step that **optionally** runs it when `llvm-as` is present.

**Stdlib Growth (written in Desi)**
* Seed modules (scope is realistic, not exhaustive):
  * `string` (split/join/replace/basic search),
  * `time` (monotonic now, sleep),
  * `fs` (read_file/write_file with safe boundaries),
  * `task` (light async helpers on top of existing futures),
  * `dataclass` decorator (sugar to define `__init__`, field defaults).
* Each module exports only **fully typed** `pub` defs; no dynamic `any`.

**Docs & EBNF updates (explicit)**
* Update `docs/syntax.md` with:
  * Default-arg syntax, evaluation rules, and restrictions.
  * `Display` trait intro and how `print` uses it.
* Update `docs/grammar.ebnf` to include `Param "=" ConstExpr` and trait decl/impl surface if needed.
* Update `docs/syntax_tour.md` with fresh examples:
  * `print("a", 5, sep=",", end="!")`,
  * defaulted function parameters,
  * simple trait impl and `to_str` usage.

**Acceptance**
* New examples compile & run:
  * `print("hello", 5, sep=",")` works without explicit `str(5)`.
  * A function with defaults can be called omitting trailing args; diags for non-trailing omissions are crisp.
* `desic build --verify-llvm` passes on the import-closure output; no duplicate symbol emission across imports.
* Docs updated (`syntax.md`, `grammar.ebnf`, `syntax_tour.md`) and consistent with behavior.
* All prior milestones remain green (`go build ./... && go test ./...`).

---

## Cross-cutting practices

* **Testing:** unit tests per package; golden tests for diagnostics & formatter; integration smoke tests for codegen.
* **Performance:** microbenchmarks for tight loops, pipelines, comprehensions; keep codegen O2-friendly.
* **Documentation:** keep `docs/grammar.ebnf` and guides canonical and updated.
* **Style:** enforced by `desifmt` (no options).
