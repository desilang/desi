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
  - **F-string interpolation semantics beyond stage-1** (currently plain strings; actual formatting is scheduled under **M14**).
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

### M13 — Project Manifest & Packaging + Call/Param Polish (✅ DONE)

**What shipped**

* **Project manifest**: `desi.mod` (DML/INI-like):
  * Parser/validator under `internal/project` with `DPM*` diagnostics.
  * `FindRoot` walks up from CWD to discover the manifest.
  * `Load` normalizes paths, editions, and diagnostics defaults.
* **Manifest-aware CLIs** (`desic` subcommands):
  * `init NAME [-p PATH] [-v VERSION] [-e EDITION] [--force]`:
    * scaffolds `desi.mod`, `src/main.desi`, and `tests/`,
    * refuses to overwrite unless `--force`.
  * `run`:
    * resolves `entry`/`roots` from the manifest (unless overridden by CLI),
    * type-checks the import graph starting at the chosen entry.
  * `build`:
    * emits textual LLVM IR to `<out_dir>/<package>.ll`,
    * `--verify-llvm` optionally runs `llvm-as` if available.
  * `test`:
    * placeholder; today shells out to `go test ./...` (real Desi test runner later).
  * **Precedence**: **CLI > env > manifest > defaults**; diagnostics defaults from `[diagnostics]` are seeded early and still overridden by flags.
* **Named arguments for function calls** (no varargs yet):
  * Surface: `f(1, 2, z=3, w=4)`; **no `**kwargs`** or splats.
  * Rule: after the first **named** argument, **no positional** arguments are allowed.
  * Resolver exports per-overload parameter names and extern metadata; checker stores per-candidate param-name vectors.
  * Checker maps call-site names → parameter indices **per overload**, then reuses the existing overload engine.
  * Borrow/move rules and FFI unsafe gate are applied on the **canonical positional vector** after mapping.
  * Pipelines (`lhs |> f(...)`) use the same overload machinery and continue to behave as in M4/M8.
* **New diagnostics (calls)**:
  * `DCA0001` — unknown named argument.
  * `DCA0002` — duplicate named argument.
  * `DCA0003` — positional argument after named arguments.
* **Behavioral guarantees**:
  * Calls that use **only positionals** behave exactly as before M13 (no surprises).
  * Named-arg failures favor **specific** `DCA*` codes; generic `DTE0101/102` are reserved for real overload/type issues.
  * FFI unsafe rule (**DFI0003**) and borrow checker diagnostics (`DBR*`) continue to fire exactly as pre-M13; named arguments are just a different way to feed the same engine.

**Deferred from M13**

* Positional varargs `*args` and related checks:
  * Collection of trailing positionals and starred lists into `list[T]`.
  * Diags like `DCA0010` (wrong element type) and `DCA0011` (`inout` via varargs forbidden).
* These are moved to **M14+** where they can be introduced together with `Display`-based printing.

**Acceptance**

* `desic init demo && cd demo && desic run` succeeds; `desic build` writes an IR file named after the package.
* Renderer flags (`--error-format`, `--color`) continue to follow `CLI > env > manifest > defaults`.
* Named arguments:
  * Canonicalize correctly for both local and imported functions.
  * Enforce `DCA0003` (“named last”) at the call site.
  * Produce `DCA0001/2` for unknown/duplicate names.
* All existing arithmetic/borrow/async/FFI tests stay green; new tests for named arguments and `DCA*` codes pass.

---

### M14 — Defaults, Display, F-Strings (Stage 2), Import-Closure IR, and Stdlib Growth (✅ DONE)

**Goals**

Deliver Python-like ergonomics without dynamic typing: default parameters, a lightweight `Display`/`Debug`-style story for stringification, robust `print`/formatting on top (including **f-strings stage 2**), import-closure IR emission, and a first useful wave of stdlib modules written in Desi.

**Language & Checker**

* **Parameter defaults**:
  * Grammar: `Param := [mode] Ident ":" Type [ "=" ConstExpr ]`.
  * Rules: defaults allowed for `move` and `ref`; **not** for `inout`. Defaults must be **compile-time const** (literals, enum variants, `len("…")` and similar constfolded intrinsics).
  * Overload selection unchanged; after selecting the overload, the checker fills omitted actuals from defaults. Arity errors/diags updated accordingly.
  * Diagnostics: `DDF0001` default not const; `DDF0002` default on `inout`; `DDF0003` missing required arg; `DDF0004` conflicting defaults across overloads (if applicable).
* **`Display` trait (or `Show`)**:
  * Prelude trait with `to_str(self) -> str`.
  * Built-in impls: `int`, `float`, `bool`, `str`, and any other core scalars we decide to expose.
  * Enables typed stringification **without** an `any`-style escape hatch.
* **F-strings (stage 2 semantics)**:
  * Reuse the existing `f"…"` lexical form from M0/M5.
  * Each `{expr}` inside an f-string is a **type-checked expression**.
  * Formatting semantics are **Rust-style**:
    * default expansion uses the `Display`-like trait,
    * a `{:?}`-style suffix (or similar) can opt into a `Debug`/repr-like trait when we add it.
  * No ES6/backtick templates; no stringly-eval. Everything stays in the typed expression world.
  * Result type is `str`; the compiler wires f-strings into `Display`/`Debug` under the hood so they compose with `print` and other APIs.
* **Varargs and `print` on top of defaults + Display**:
  * Language-level support for positional varargs `*args` on the last parameter:
    * `*p:T` in the signature is treated as `list[T]` at the type level.
    * Call-site `*Expr` must type-check to `list[T]`.
    * Diagnostics: `DCA0010` wrong vararg element type; `DCA0011` `inout` through varargs forbidden.
  * Public `print` API built on this:
    * `def print(*args: list[Display], sep: str = " ", end: str = "\n") -> int`.
    * Internals: `print_core(args: list[str], sep: str, end: str) -> int` + wrapper that maps `Display`→`str`.
  * Examples and tests updated to rely on:
    * implicit stringification via `Display`,
    * f-strings for more complex formatting (`f"total = {x}"` etc.).

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
  * `Display` trait intro and how `print` and f-strings use it.
  * Varargs rules, including diagnostics for bad `*args` uses.
* Update `docs/grammar.ebnf` to include `Param "=" ConstExpr`, trait decl/impl surface if needed, varargs, and f-string interpolation notes.
* Update `docs/syntax_tour.md` with fresh examples:
  * `print("a", 5, sep=",", end="!")`,
  * functions with defaults,
  * simple trait impl and `to_str` usage,
  * f-strings such as `f"total is {total}"` and `f"value = {x:?}"`.

**Acceptance**

* New examples compile & run:
  * `print("hello", 5, sep=",")` works without explicit `str(5)`.
  * A function with defaults can be called omitting trailing args; diags for non-trailing omissions are crisp.
  * F-strings type-check embedded expressions and expand via `Display`/`Debug` traits.
* `desic build --verify-llvm` passes on the import-closure output; no duplicate symbol emission across imports.
* Docs updated (`syntax.md`, `grammar.ebnf`, `syntax_tour.md`) and consistent with behavior.
* All prior milestones remain green (`go build ./... && go test ./...`).

---

### M15 — Error Handling: try/except/finally/raise (✅ DONE)

**What shipped (Phase 1 — syntax + lowerer scaffolding, ✅)**

* **AST nodes**: `TryStmt` (Body, Except, Finally, ExceptVar, ExceptType) and `RaiseStmt` (Value).
* **Parser**: `parseTry()` / `parseRaise()` with Python-style syntax:
  * `try:` body, `except [Type] [as var]:` handler, `finally:` block.
* **Type checker**: recursive checking of try/except/finally blocks; except variable bound as `str`.
* **Lowerer**: `TryStmt` creates except/continuation/finally blocks with scope management.
  * `TryExpr` (`?` operator) is **context-aware** — inside a try block, Err redirects to the except handler instead of early return.
  * `RaiseStmt` calls `__desi_panic(msg)` + `ret` (exits process).
* **HIR**: `Jump` instruction for unconditional block transfers.
* **LLVM backend**: `Jump` handler emits `br label %target`.
* **Runtime**: `__desi_panic(const char* msg)` in `builtins.c`.

**Phase 2 — Catchable exceptions (Planned)**

* **Mechanism**: `setjmp`/`longjmp` for non-local jumps.
  * `try:` → `setjmp(buf)` saves execution state (~5-10ns overhead per try entry).
  * `raise Exc(msg)` → stores error, `longjmp(buf, 1)` unwinds to handler.
  * `except Exc as e:` → checks error type tag, binds variable.
  * **No overhead outside try blocks.** Normal code pays zero cost.
* **Exception hierarchy** (10 built-in types):
  * `Exception` (base), `ValueError`, `KeyError`, `IndexError`,
    `ZeroDivisionError`, `IOError`, `RuntimeError`, `OverflowError`,
    `TimeoutError`, `ConnectionError`.
* **Runtime**: new `runtime/exception.c` with `__desi_raise`, `__desi_try_push/pop`,
  `__desi_get_exception`, `ExceptionFrame` stack.
* Wire `__panic_divzero` → `__desi_raise(ZeroDivisionError)` to make it catchable.

> **Upgrade path (v0.2.0+):** replace `setjmp/longjmp` with **LLVM zero-cost exceptions**
> (`invoke`/`landingpad`). This gives zero overhead on the happy path at the cost of
> ~1-5μs on throw. The user-facing syntax stays the same — only the backend changes.

---

### M16 — Performance Tooling: Advisor, `@perf`, Runtime AST (✅ DONE)

**Goals**

Deliver a zero-overhead performance analysis module with three components: a static analysis advisor that catches algorithmic anti-patterns at compile time, a `@perf` decorator for runtime benchmarking, and a runtime `ast` library for user-built analysis tools.

**Static Analysis — Performance Advisor**

* **Diagnostic codes**: `DPR` prefix (Desi PeRformance).
  * `DPR0001` — O(n²) nested loop detected.
  * `DPR0002` — String concatenation in loop (recommend `list.join()`).
  * `DPR0003` — Repeated collection lookup in loop (recommend `set`/`dict`).
  * `DPR0004` — Unbounded allocation in loop (recommend size hints).
* **Three analysis levels**: `relaxed`, `default`, `strict`.
  * Configured via `[diagnostics] perf = "default"` in `desi.mod`.
* **Implementation**: Go-based rules in `internal/check/check_perf.go`.
  * Advisor rules are pluggable functions, not hardcoded in the core type checker.
  * Each rule receives a `*ast.FuncDecl` and emits diagnostics.

**Runtime Benchmarking — `@perf` Decorator**

* **Macro protocol**: Registered as a Tier 2 `MacroProtocol` (not a builtin decorator).
  * `strip_in_release = true` — automatically removed from release builds.
* **Decorator API**: `@perf(iterations=N)` on any function.
  * Wraps function body with `__perf_bench_start()`/`__perf_bench_finish()` calls.
  * Prints timing statistics: total, average, min, max.
* **Runtime**: `runtime/perf.c` with high-resolution timing (`clock_gettime` / `mach_absolute_time`).
* **Stdlib**: `lib/perf/__mod.desi` with safe Desi wrappers.

**CLI — `desic perf` Subcommand**

* Discovers and runs all `@perf`-decorated functions in the project.
* Accepts `--level=relaxed|default|strict` to control advisor strictness.
* Modeled after `testCmd` in the existing CLI infrastructure.

**Runtime `ast` Library**

* **`import ast`**: Parse `.desi` source files into walk-able AST trees at runtime.
* Uses the same parser the compiler uses internally (exposed via C bindings).
* Enables users to build custom linters, code generators, documentation tools.
* Stable public types: `ast.FuncDecl`, `ast.ForStmt`, `ast.CallExpr`, `ast.Ident`, etc.
* **This is the v0.1.0 foundation** for the v0.2.0 compile-time macro introspection.

**Acceptance**

* `DPR` diagnostics fire correctly for known anti-patterns (nested loops, string concat in loop).
* `@perf` functions produce timing output when run via `desic perf`.
* `@perf` code is absent from `desic build --release` output.
* `import ast` can parse a `.desi` file and walk its tree at runtime.
* Advisor level is configurable via `desi.mod` and `--level` CLI flag.
* All prior milestones remain green.

---

### M17 — Build Audit & Permissions (✅ DONE)

**Goals**

Implement a compile-time security audit system that scans the full import graph for sensitive API usage and enforces project-level permissions policies.

**Manifest — `[permissions]` Section**

* New section in `desi.mod`:
  ```toml
  [permissions]
  allow = ["fs", "http", "json"]
  deny = ["shell", "process"]
  audit = true
  ```
* **`allow`**: Whitelist of permitted API modules. If set, any module not listed triggers a warning.
* **`deny`**: Blacklist of forbidden API modules. If any dependency uses a denied module, the build fails.
* **`audit`**: When `true`, print a full audit report before building.
* Parsed and validated by `internal/project` manifest loader with `DPM*` diagnostics.

**API Sensitivity Tiers**

* Classification table in `internal/check/api_tiers.go`:
  * **Tier 0 — Safe**: `math`, `string`, `json`, `re`, `collections`, `time`, `log`.
  * **Tier 1 — System**: `fs`, `os`, `path`, `env`, `args`, `process`, `shell`.
  * **Tier 2 — Network**: `http`, `net`, `db`, `redis`, `websocket`, `smtp`.
  * **Tier 3 — Privileged**: `unsafe` blocks, raw pointers, `@extern` FFI.
* Leverages existing `StdlibImports map[string]bool` in `check/info.go` for import tracking.

**Audit Report**

* Generated during the `desic build` pipeline after import resolution.
* Lists every imported module per dependency with its sensitivity tier.
* Highlights denied modules and blocks the build with actionable error messages.
* Machine-readable output via `--error-format=json`.

**Transitive Scanning**

* The resolver already walks the full import graph.
* Audit adds a post-resolution pass that collects all `StdlibImports` across all modules (including third-party).
* Reports are per-dependency, showing which file and line uses each sensitive API.

**Acceptance**

* `[permissions]` section parses correctly with `DPM*` diagnostics for invalid values.
* `deny = ["shell"]` causes `desic build` to fail if any dependency imports `shell`.
* Audit report correctly identifies transitive dependencies and their API usage.
* `--error-format=json` produces machine-readable audit output.
* All prior milestones remain green.

---

## v0.1.x — Move checks from run time to compile time

> **Status:** planned, measured, not started unless noted.
> **Why one section:** every item here is the same move — something Desi
> currently checks or arranges *while a program runs*, which the compiler could
> settle *before* it does. That is simultaneously the performance plan and the
> memory-safety plan, because a runtime check is a permanent tax and a
> compile-time proof is free forever.

### Where v0.1.0 leaves off

Benchmarks are whole-process wall times; subtract the `startup` row from both
sides before reading any of them. Linux, release, 2026-08-02:

| | Desi | C | reading |
|---|---|---|---|
| string churn | 11 | 14 | Desi ahead |
| loop arithmetic, string build | 1 | 1 | tied |
| quicksort, binary tree | 2–3 | 1–2 | close |
| dict ops | 8 | 5 | close |
| recursive calls | 9 | 6 | close (was 23) |
| **list ops** | **10** | **3** | behind |
| **allocation churn** | **13** | **1** | behind |

Both laggards are dominated by allocating small objects. Nothing else in the
table is far off, and the call-overhead gap closed in v0.1.0 by replacing the
frame counter with a stack headroom check.

---

### A. Escape analysis: stop the false positives

**Problem.** Arena promotion is built and works, but does not fire on the
commonest shape in real code — accumulate a number in a loop while touching a
collection. These two loops differ only in the last line:

```desi
let t = [i, i, i]
print(str(len(t)))          # promoted to the arena
```
```desi
let t = [i, i, i]
total := total + len(t)     # falls back to malloc per iteration
```

`escape.go` marks **every symbol on the right-hand side** as escaping when the
left-hand side is not itself an arena candidate. `total` is an `int`, so it is
not a candidate, and `t` is tarred by association — even though `len(t)` hands
over an integer and the list goes nowhere.

**Fix.** Consult the type. If the assignment target cannot hold a reference —
`int`, `float`, `bool` — then nothing on the right escapes through it.

**Cost.** 1–2 days. Low risk: the change only ever *removes* false escapes.
Marking too little is the dangerous direction and is untouched.

**Payoff.** Should fix `alloc_churn` outright, and every loop shaped like it.

---

### B. Per-iteration arena reset

**Problem.** An arena currently lives for the whole function. A loop that
allocates half a million times would grow it half a million times, so loop-local
collections cannot simply be promoted and forgotten.

**Fix.** Reset the arena at the end of each iteration, so a loop body's
allocations cost a bump-pointer increment and one reset. This is "Phase 2:
Scope-Based Arenas" in [hybrid_memory_management.md](todo/hybrid_memory_management.md),
which is **not** built despite that document's summary.

**Cost.** 3–5 days, and the one to test hardest. Resetting an arena while
something still points into it is precisely the bug class the design exists to
prevent, so the escape analysis must be right *first* — B depends on A.

**Payoff.** With A, brings `alloc_churn` near C.

---

### C. Collection element access (not v0.1.x)

`list_ops` appends a million integers one at a time. The gap is not allocation
strategy: each `append` is a runtime call that checks capacity and may grow,
against C's inlined array store. Closing it means inlining `append`'s fast path
into the caller, or specialising `list[int]` so elements are stored unboxed.

**Cost.** 1–2 weeks, and it changes how collections are represented. Deferred.
Until then `list_ops` stays behind, and the benchmark README should say so
rather than imply otherwise.

---

### D. Require field initialisation at compile time

**Problem.** Every class instance is zeroed at construction so that a field the
constructor never assigns reads as `0` rather than as whatever the allocator had
lying around. That closes a real hole — it was returning garbage before v0.1.0 —
but it is the Go answer, not the Rust one, and it costs a `memset` per
construction forever.

**Fix.** Have the checker reject a program that reads a field no constructor
assigns and no declaration defaults. Then delete `__desi_zero`.

**Cost.** 2–3 days. **Breaking**: code relying on implicit zeroing stops
compiling, which is why this belongs early in 0.1.x rather than later.

**Payoff.** Strictly safer *and* strictly faster — the runtime cost disappears
because the compiler proved it unnecessary. The clearest example in the whole
list of why this section exists.

---

### E. Narrow "leak rather than crash"

Every boundary listed in [memory-model.md](../memory-model.md) is a place the
compiler could not decide who owned a value and chose to leak instead of risking
a free. Each one narrowed returns memory *and* speed. Incremental, no cliff,
good background work across 0.1.x.

---

### F. Elide the print lock when a program has no tasks

`print` takes a lock so concurrent output cannot interleave mid-line. A program
that never spawns a task cannot race, and the compiler can see that. Half a day,
same shape as the guard elision in v0.1.0.

---

### G. Monomorphise generics (not v0.1.x)

A generic function currently boxes its argument on the heap and hands back a
pointer. Rust generates a specialised copy per type: no allocation, and a whole
class of bug cannot exist — several were fixed in v0.1.0 that existed *because*
of the boxing.

**Cost.** 1–2 weeks; a codegen strategy change. Deferred.

---

### Not planned: a borrow checker

The actual Rust mechanism is a months-long language-design project, and it would
change what Desi feels like to write — the strictness is the trade. Worth
deciding deliberately, not under release pressure.

### Order

A → D → B, then E and F as they fit. C and G are 0.2.0 material.

Realistic outcome for 0.1.x: `alloc_churn` near C, uninitialised fields
impossible at compile time instead of papered over at runtime, one runtime cost
deleted outright, and `list_ops` still behind — documented, not implied away.

---

## v0.2.0 Vision — Compile-Time Macro Introspection

> **Status:** Design complete, implementation deferred to v0.2.0.
> **Full design:** [compile_time_macros.md](todo/compile_time_macros.md)

The headline feature for v0.2.0: **write macro rules in Desi that execute during compilation with full AST access.**

### What this enables
* User-defined compile-time analyzers (SQL injection detection, API contract validation).
* Community-contributed advisor rules without modifying the Go compiler.
* Domain-specific linters that run as part of the standard build.

### Why deferred from v0.1.0
* Requires embedding a Desi tree-walking interpreter in the Go compiler (4-6 weeks).
* AST type API must be stable before exposure — v0.1.0 stabilizes the shapes.
* Interpreter bugs could crash the compiler — unacceptable for launch quality.

### Implementation path
* **v0.1.0**: Runtime `ast` library + declarative macros (foundation).
* **v0.2.0**: Tree-walking interpreter with sandboxing, `USR` diagnostic prefix.
* **v0.3.0**: Stable AST API, remove `@experimental` flag.

---

## Distributed Systems (v0.2.0+ Vision)

> **Status:** Research phase. Full design doc: [distributed_systems.md](todo/distributed_systems.md)

Erlang-inspired distributed actor/messaging layer built on Desi's existing concurrency primitives (channels, supervisors). Native-compiled (no VM), so this would be a TCP-based messaging protocol rather than BEAM-style location transparency.

### Key components
* **Node connection** — `distributed.start_node()`, `distributed.connect()`
* **Cross-node messaging** — `distributed.send()` / `distributed.recv()`
* **Distributed supervisor** — Remote `start_child()` with crash recovery across nodes
* **Wire protocol** — Binary serialization, shared-secret auth, heartbeat

---

## Cross-cutting practices

* **Testing:** unit tests per package; golden tests for diagnostics & formatter; integration smoke tests for codegen.
* **Performance:** microbenchmarks for tight loops, pipelines, comprehensions; keep codegen O2-friendly.
* **Security:** build audit scans transitive imports; `[permissions]` enforced in CI.
* **Documentation:** keep `docs/grammar.ebnf` and guides canonical and updated.
* **Style:** enforced by `desifmt` (no options).
