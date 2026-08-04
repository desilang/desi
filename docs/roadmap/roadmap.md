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

### A + B. Arena promotion and per-iteration reuse — ✅ SHIPPED

> **Shipped together in `594e7d47`**, which is the only way they work: A alone
> was measured at 30 ms / 55.1 MB on `alloc_churn` against 13 ms / 1.8 MB
> before it, and was reverted. The sections below are the original plan; the
> outcome, including a third condition neither of them anticipated, is recorded
> under "What actually happened" after item B.

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

**Cost.** 1–2 days. Low risk *for correctness*: the change only ever removes
false escapes, and marking too little is the dangerous direction, which is
untouched.

**Payoff — measured, and it is negative on its own.** This was implemented and
benchmarked on the `perf-recursion-guard` branch, then reverted. The analysis
change works exactly as intended: `alloc_churn`'s loop-local list stops emitting
`list_new` and starts emitting `list_new_in`, i.e. it promotes to the arena. The
result (Linux, release):

| | before A | with A | C |
|---|---|---|---|
| `alloc_churn` time | 13 ms | **30 ms** | 1 ms |
| `alloc_churn` peak RSS | 1.8 MB | **55.1 MB** | 1.5 MB |
| `list_ops` | 10 ms / 9.5 MB | 16 ms / 17.4 MB | 4 ms / 5.3 MB |
| `dict_ops` | 8 ms / 9.0 MB | 9 ms / 11.8 MB | 6 ms / 5.3 MB |

Output stayed correct everywhere; this is a performance regression, not a
miscompile. The cause is structural: **the arena is function-scoped and is only
destroyed when the function returns.** Promoting an allocation that happens
500,000 times inside a loop therefore replaces 500,000 short-lived `malloc`/
`free` pairs — which a modern allocator serves out of a hot free list — with
500,000 live arena slots that are never reclaimed until the function exits. The
arena grows to hold all of them at once, and growing it costs more than the
allocator it replaced.

So A is **not independently shippable**. It is a prerequisite for B that makes
things worse until B lands. Do not merge A on its own.

---

### B. Per-iteration arena reset

**Problem.** An arena currently lives for the whole function. A loop that
allocates half a million times would grow it half a million times, so loop-local
collections cannot simply be promoted and forgotten.

**Fix.** Reset the arena at the end of each iteration, so a loop body's
allocations cost a bump-pointer increment and one reset. This is "Phase 2:
Scope-Based Arenas" in [hybrid_memory_management.md](todo/hybrid_memory_management.md),
which is **not** built despite that document's summary.

**Cost — revised upward to 1–2 weeks** after implementing A and looking hard at
what B actually requires. The original 3–5 day estimate assumed "call
`__arena_reset` at the loop latch". That is wrong twice over:

1. **Reset is too blunt; it needs mark/rewind.** A function's arena also holds
   allocations made *before* the loop. Resetting it at the latch frees those
   too. What is needed is a mark taken at loop entry and a rewind to that mark
   at the latch, so only the current iteration's allocations are reclaimed. That
   is a runtime addition (`__arena_mark` / `__arena_rewind`), not just a call.

2. **Rewinding safely needs *iteration-scoped* escape information, which the
   current analysis cannot express.** `escape.go` answers one question: does
   this value outlive the *function*? Rewinding asks a strictly finer one: does
   it outlive the *iteration*? A value can be arena-safe by the first measure
   and unsafe by the second:

   ```desi
   let mut keep = []
   for i in range(10):
       let t = [i]
       keep.append(t)   # t outlives its iteration, but not the function
   ```

   Neither `keep` nor `t` escapes the function, so today both are arena
   candidates. Rewinding at the latch would leave `keep` holding ten dangling
   pointers — silent memory corruption, and precisely the bug class the whole
   design exists to prevent.

   The fix is to give the analysis loop scope: track which loop body each
   candidate is declared in, and treat a candidate as iteration-escaping if the
   deps graph flows it into any symbol declared outside that body. The data is
   mostly there (`deps` already records the edges); what is missing is the
   notion of declaration scope, plus emitting the mark/rewind only for loops
   where *every* body-local candidate clears that test.

**Payoff.** With A, brings `alloc_churn` near C. Without B, A is a regression —
see the table above.

**Testing bar.** Higher than anything else in this list. A wrong answer here
does not produce a failing test, it produces a use-after-free that happens to
work until it doesn't. At minimum: the `keep.append(t)` shape above and its
variants (store into a field, into a dict, into an outer tuple, return from
inside the loop, `break` carrying a reference out) must each be shown to *not*
get a rewind, as compiler tests over emitted IR rather than as behaviour tests.

---

### What actually happened with A + B

Shipped, with a third condition neither item predicted.

**A collection that grows must stay on the heap.** `realloc` releases or extends
the block it replaces; a bump allocator can only hand out a new one and abandon
the old. A list appended to a million times doubles about twenty times, and in
an arena every one of those dead backing arrays is still there at the end. That
is what took `list_ops` from 9.5 MB to 17.4 MB — A promoting a list it had no
business promoting. Any method call on a collection, and any assignment through
an index, now keeps it on the heap.

**B's eligibility rule came out simpler than feared.** It tests the dependency
edges the escape constraints already build: a loop may rewind only if nothing
declared inside it flows to anything declared outside. The test is on the edge
itself rather than on whether an arena pointer travels along it, because
tracking that across heap containers means following chains, and a missed link
is a use-after-free rather than a missed optimisation.

**`list_new_in` was also splitting its allocation.** It made two bump calls,
header then data, where `list_new` had long since packed both into one malloc
and used the inline-data layout. Putting them back together took `alloc_churn`
from 17 ms to 14 ms.

**Results** (Linux, release, against the pre-A baseline):

| | before A | shipped | C |
|---|---|---|---|
| `alloc_churn` | 13 ms / 1.8 MB | **14 ms / 1.7 MB** | 1 ms / 1.5 MB |
| `list_ops` | 10 ms / 9.5 MB | **12 ms / 9.5 MB** | 4 ms / 5.3 MB |
| `dict_ops` | 8 ms / 9.0 MB | **12 ms / 9.0 MB** | 6 ms / 5.3 MB |

**Be honest about this: it is not the win the section predicted.** Memory is
neutral-to-slightly-better and time is within this machine's noise. The
prediction was "brings `alloc_churn` near C", and `alloc_churn` is still 14 ms
against C's 1 ms. glibc's tcache already serves a repeated small alloc/free
about as fast as a bump allocator can, so there was less on the table than the
plan assumed.

What the work did buy: the 55 MB cliff is gone, the arena no longer grows
without bound in a loop, and growing collections no longer land in it at all —
so the arena is now safe to leave switched on, which it was not before.

**The remaining `alloc_churn` gap is not allocation.** It is the call per
`list_append` and per `list_get`, which is item C below.

**Testing.** `escape_loop_test.go` covers which loops qualify;
`arena_rewind_test.go` covers the decision reaching the generated code;
`compiler/runtime/tests/arena_mark.c` covers the runtime's invariants. The
example suite was run on both legs with `-DDESI_ARENA_POISON`, which scribbles
over every byte a rewind releases — AddressSanitizer is blind here, because the
arena is one large allocation and reusing bytes inside it is invisible to it.

---

### C. Collection element access — ✅ SHIPPED

`list_ops` appends a million integers one at a time. Each `append` and each read
was a runtime call, against C's inlined array store.

**Measured first, and the measurement redirected the work twice.**

A C decomposition of 1M appends + 1M reads:

| | |
|---|---|
| typed `int64[]`, inlined — what C does | 0.76 ms |
| `void*` slots, inlined | 0.74 ms |
| `void*` slots, minimal out-of-line calls | 1.73 ms |
| real `list.c`, out-of-line | 3.27 ms |

**The pointer-slot representation is free for anything pointer-sized** — 0.74
against 0.76. This section used to propose "specialising `list[int]` so elements
are stored unboxed" as a route. It is not one: an `int64` and a `void*` are both
eight bytes in a contiguous array, and Desi already stuffs the int straight into
the slot. The whole gap was the call.

**But inlining the call naively makes it worse**: 4.22 ms against 3.28. Both
`list_get` and `list_append` end in `fprintf(stderr, …)` on their failure paths,
and inlining drags a varargs call into the loop body. Adding `list.c` to the LTO
hot set — a one-line change, and the obvious first move — would have been a
regression.

What works is the shape Rust's `Vec` uses: a small hot path, branching away to
something cold. Same code, same semantics, error paths marked `noinline` and
`cold`: **1.09 ms against 3.28**.

**Shipped as IR emission rather than LTO.** The backend emits the check and the
load/store inline and calls the runtime only on failure or growth. That was
chosen over restructuring `list.c` plus LTO because LTO is not dependable:
Windows has it but never covered `list.c`, Unix release has none at all, and
linking bitcode needs an `lld` or gold plugin that is not always installed.
Emitting in IR behaves identically everywhere.

Results, Linux release, old and new compilers back-to-back:

| | before | after | C |
|---|---|---|---|
| `list_ops` | 10–15 ms | **6 ms** | 4–6 ms |
| `alloc_churn` | 13–16 ms | **8–9 ms** | 2 ms |
| `matrix_mul` | 4–5 ms | **3 ms** | 1 ms |

`list_ops` is level with C. `alloc_churn`'s remainder is the allocation itself,
not element access.

**This made `DesiList`'s layout an ABI.** The backend reads `data`, `length`,
`capacity` and `type_tag` at fixed offsets. `list.h` asserts them,
`list_layout.go` names them, and `list_layout_test.go` compiles a probe against
the real header and compares — verified to fire.

**Still to do here:** `dict` and `set` get their elements the same way and would
take the same treatment. `dict.c` is already in the Windows LTO set, so measure
before assuming the gap is the same shape.

---

### C2. Stop boxing floats — ✅ SHIPPED

> Shipped in `9b84678c`. It came in far smaller than this entry predicted: the
> boxing and the unboxing turned out to be two adjacent branches of one
> conversion switch in `emit_func.go`. The plan below is left as written because
> the survey of call sites was accurate and is worth keeping for the next
> representation change.
>
> **The old comment in that switch said a float cannot be bitcast to a pointer.**
> True, and why it reached for `malloc`. But it can be bitcast to an integer of
> the same width, and integers were already going into slots. That was the whole
> fix.
>
> **It rippled much further than the float benchmark.** `binary_tree` and
> `quicksort` both reached parity with C without being touched — the boxing was
> most of what they were measuring — and `matrix_mul` went 3 ms to 2 ms.
> `alloc_churn` went 8–9 ms to 6 ms. The float probe went from 12.5 MB to
> 1.9 MB, flat with the equivalent int loop.
>
> **It deleted more than it added.** `list_set` no longer frees the old element,
> `list_copy`'s float branch no longer clones, and `list_free_elems` — added
> three commits earlier to mop up exactly this leak — went with it, along with
> the scope-exit call the lowerer emitted for arena lists. `memory-model.md` now
> says collections of numbers leak nothing, with no qualification.
>
> Validated on Linux as well as Windows: 525/525 both legs on each, which
> matters because Linux uses clang rather than MSVC and has no LTO, so the
> bitcast path is confirmed independently of the Windows toolchain.

**Original plan follows.**

A list element is a pointer-sized slot. Ints go in directly. A `float` does not:
each one is a separate `malloc` holding the double, and the slot holds a pointer
to it. Measured over 1M appends + reads:

| | |
|---|---|
| boxed slots, via calls — Desi's `list[float]` | 28.65 ms |
| real `double[]`, inlined — Rust's `Vec<f64>` | 1.18 ms |

**24x.** It is also the cause of a leak that `list_free_elems` currently mops up
after: the boxes are `malloc`'d, so an arena list of floats leaks one per
element until something frees them. Fixing the representation deletes that
problem rather than managing it.

**The fix.** A `double` is eight bytes and so is a slot on every target Desi
supports, so the bits go straight in, exactly as ints already do — bitcast on
the way in, bitcast on the way out. No allocation, no indirection, no leak.

**Where the work is.** Lowering creates the box today (the runtime receives an
already-boxed pointer — see `list_clone_elem`). So:

1. Lowering: emit a bitcast into the slot instead of a `malloc` and store.
2. `list.c`: roughly ten `DESI_TAG_FLOAT` special cases — `list_clone_elem`,
   `list_set`, `list_copy`, `list_extend`, the slice and map/filter builders —
   become plain slot copies.
3. Delete `desi_free_float_elems`, `list_free_elems`, and the scope-exit call
   the lowerer emits for arena lists. The leak class goes with them.
4. Guard the assumption: `_Static_assert(sizeof(double) <= sizeof(void*))`, next
   to the layout assertions that already exist.
5. Check `dict` and `set` for the same pattern before assuming lists are the
   only place it appears.

Net this **removes** more code than it adds. The risk is breadth, not depth:
every float path needs re-validating, and the float examples in the suite are
the ones to watch.

**Payoff.** 24x on float-heavy code, one documented leak boundary closed
outright, and `memory-model.md` gets simpler rather than more qualified.

---

### D. Require field initialisation at compile time

**Problem.** Every class instance is zeroed at construction so that a field the
constructor never assigns reads as `0` rather than as whatever the allocator had
lying around. That closes a real hole — it was returning garbage before v0.1.0 —
but it is the Go answer, not the Rust one, and it costs a `memset` per
construction forever.

**Status: the elision half is done. The language change is not, and should not
be attempted as scoped here.**

**What shipped.** A definite-assignment pass over `__new__`
([field_init.go](../../compiler/internal/check/field_init.go)). When every
constructor on a class writes every field on every path out — early returns,
returns inside loops, and raises included, since a half-built instance can still
reach a drop path — lowering skips the zeroing. That removed **96 of 264**
zeroing call sites across the example suite, about a third, with no language
change and nothing to migrate. The analysis only ever fails safe: anything it
cannot follow keeps the memset.

**What did not, and why the original plan was wrong.** "Delete `__desi_zero`"
assumed implicit zeroing was a papering-over. It is not: it is the defined
behaviour of a class declared with no `__new__`, which is a documented Desi
shape —

```desi
class Point:
	pub mut x: int
	pub mut y: int

let mut p = Point()   # zero-initialised, then assigned from outside
p.x := 10
```

**81 example classes and 9 in `compiler/lib` are written this way.** Rejecting
them needs somewhere else for the initial value to come from, and `FieldDecl`
has no default-value slot — so this is not a checker change, it is new surface
syntax (`x: int = 0`) through the parser, checker, and lowerer, plus migrating
every one of those classes.

**Revised cost.** The elision: done. The language change: 1–2 weeks including
field-default syntax and the migration, and it is **0.2.0 material** — a
breaking change to how classes are written is not something to land days before
a first release.

**Payoff, realised.** A third of the construction memsets gone, and a new
compile-time notion of "this constructor is complete" that the field-default
work can build on later.

---

### E. Narrow "leak rather than crash"

Every boundary listed in [memory-model.md](../memory-model.md) is a place the
compiler could not decide who owned a value and chose to leak instead of risking
a free. Each one narrowed returns memory *and* speed. Incremental, no cliff,
good background work across 0.1.x.

---

### F. Elide the print lock when a program has no tasks — ✅ SHIPPED, differently

`print` takes a lock so concurrent output cannot interleave mid-line. A program
with only one thread cannot race, so the lock can go.

**The compile-time version proposed here is unsound.** "This program contains no
`spawn`" does not mean no threads: `future.c`, `scheduler.c`, `supervisor.c` and
`websocket.c` all start threads from code a program reaches through `async`,
`http.serve`, or a supervisor without ever writing `spawn`. Proving those
unreachable is a whole-program call-graph question, and being wrong tears output
lines.

**So it is a runtime flag instead**, set before any thread starts and never
cleared. Measured, per lock/unlock pair:

| | |
|---|---|
| mutex pair (what every print paid) | 13.87 ns |
| flag load (what it pays now) | 1.73 ns |
| nothing (what compile-time elision would pay) | 1.78 ns |

The flag captures the entire saving — compile-time elision is not measurably
better than a predictable branch on a hot global, and it is the version that can
be wrong. It also wins in cases the compile-time test would have given up on: a
program that does spawn eventually still gets the fast path until it does.

**Payoff in context: small.** A `print` costs about 516 ns, so this is ~2.3% of
a print and nothing at all for code that does not print in a loop. It moves no
benchmark. Worth having — it is free and always correct — but it is not
performance work in the sense the top of this section means.

`runtime_thread_flag_test.go` fails if a runtime source file creates a thread
without calling `__desi_note_thread_start()` first, so the obligation does not
quietly rot.

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

**D (done) → A+B (done) → F (done) → C (done) → C2 (done) → stack-allocate
collections and objects → dict/set fast paths → OTP audit → data-race design →
analyses onto the CFG.** G is 0.2.0 material,
and so is D's language half.

This is a change from the original A → D → B. A was implemented first, measured,
and reverted: it is a regression on its own (see the table under A), so **A and B
have to land as one piece of work, or neither**. D went next because it was the
only item with no dependency on the arena work — and it split cleanly in two,
with the compile-time elision shipping and the breaking language change deferred
to 0.2.0.

A+B landed, and came in well under the 1.5–2.5 week estimate — most of that
estimate was for an iteration-scoped analysis that turned out to be expressible
in the dependency edges already there. What it did not buy is the speed the
section promised; see "What actually happened" above.

Outcome for 0.1.x so far: a third of the construction memsets proved
unnecessary and removed, the arena made safe to leave on in loops, and
`alloc_churn` / `list_ops` still well behind C — documented, not implied away.
E and F remain.

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
