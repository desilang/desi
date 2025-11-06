# Desi Compiler Roadmap (Revised, LLVM-first)

**Goals**
Pythonic surface • Rust-like safety/diagnostics • Elixir-style async • C/Rust/Java-class performance.
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
**Phase-4 (✅ — Ergonomics v1):**
- **4a:** Implicit `str` on `+` when one side is `str` (primitive-friendly rule).
- **4b:** **f-strings (stage 1)** — recognized as `str` literals; interpolation deferred.
- **4c:** Tuple unpacking via multi-LHS already works; `for`-target destructuring deferred.
- **4d:** Slice steps `s[i:j:k]` + short forms parsed; **string slices type to `str`**.

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

* **HIR skeleton** with **named temporaries** and a pretty-printer:
  * `Module/Func/Block`, `Let`, `Assign`, `Call`, `Ret`, `If`, `While`, `Drop`.
  * Resource ops: `DecRef{val}`, `ArenaAlloc{arena,…,dst}`, `DestroyArena{arena}`.
* **Deterministic drops (RAII)**:
  * Block-local liveness + scope exit handling; shadowing drops old binding.
  * `using arena:` lowers to a single `DestroyArena` at scope end (and on early return).
  * For `rc/arc[T]` shapes, `Drop` lowers to **`DecRef`**.
* **CLI emitter**: `desic emit-ir <file.desi>`
  * Parse → find `def main()` → **AST→HIR** (source-aware string literals) → **emit textual LLVM IR**.
  * Built-in `print("…")` lowers to `puts` with private `@.str.N` globals.
  * Emits conservative `llvm.lifetime.start/end` for stack locals.
  * Emits declarations **on demand** when `DecRef`/`DestroyArena` are used:
    * `declare void @__rc_dec(ptr)`
    * `declare void @__arena_destroy(ptr)`

**Acceptance**

* `go build ./... && go test ./...` green, including:
  * Lowering tests for deterministic **Drop**/`DecRef` placement and `using arena` epilogue.
  * Backend tests for **lifetime intrinsics**, **puts/string globals**, and **DecRef/DestroyArena** stubs.
* Running `desic emit-ir examples/14_m7_main.desi` produces valid `.ll` for a minimal `main`:
  * string global + `puts` call + `ret i32 0`.
* **Note:** Tier-0 ships **textual IR emission** only. Object/exec build or JIT is deferred to later milestones.

---

### M8 — Async & Futures (✅ DONE)

**What shipped**

* **Async lowering to state machines** with a **wrapper** (`@name` returning a future handle) and a **poll** function (`@name$poll`) that now **threads a real frame parameter**.
  * HIR gained **function parameters** (`Func.Params`) and **frame sugar ops**: `frame.set` / `frame.get` for saving/restoring locals across suspension points.
  * The async lowerer saves live locals before `await`, sets a state, returns `false`, and **restores** on resume using `frame.get`.
* **`await` = suspension barrier** is enforced at lowering time:
  * Attempting to hold an **`inout` (unique)** borrow **across `await`** produces a clear **DBR0001** diagnostic wired to the catalog.
* **Async lambda** support end-to-end:
  * `async lambda ...` **desugars** to a hidden `async def __lam$N(...)` and then lowers through the same wrapper+poll pipeline.
* **Backend / IR polish**
  * LLVM emitter prints params (Tier-0 all as `ptr`) and recognizes async wrappers (return **`ptr`**).
  * `__future_register_poll(fut, &name$poll, frame)` now passes the **frame** argument.
  * Stable SSA aliasing for simple lets **and** frame slots; temp counter monotonicity fixed.
  * **Lifetime placement:** `llvm.lifetime.end` is emitted **immediately before `ret`** and **never after** it.
* **Tier-0 runtime stubs** remain textual:
  * `await` in synchronous contexts lowers to a blocking stub (`__await_blocking`).
  * Futures use `__future_new`, `__future_complete`, and `__future_register_poll`.

**Tests / examples**

* Lowering: `async_lower_two_awaits_test.go`, `async_lower_frame_test.go` (save/restore locals), `await_barrier_test.go` (DBR0001), `async_lambda_test.go`.
* Backend: `emit_async_stubs_test.go`, `emit_register_poll_test.go`, lifetime intrinsic tests.
* Examples: `examples/15_m8_async_basic.desi`, `examples/16_m8_async_lambda.desi`.

**Acceptance**

* `go build ./... && go test ./...` green.
* HIR shows **`poll(%frame)`** and `frame.set/get` around each `await`; two-await case restores locals correctly.
* Async lambdas compile via desugaring; **`inout` across `await`** is rejected with a clear diagnostic.
* Tier-0 IR is cleaner (pointer-return wrappers, stable temps, and **no lifetime.end after ret**).

---

### M9 — FFI v1 (C ABI, ✅ DONE)

**What shipped**

* **Types (Tier-0)**: `usize`, `isize` (lower to **`i64`**), **`cptr[T]`** (lower to **`ptr`**; `T` ignored at ABI layer).
  * Pretty-printing and basic arithmetic sanity for `usize`/`isize`.
  * `cptr[T]` comparable to `none` for null checks.
* **`unsafe` blocks**:
  * New statement `unsafe:` (AST + parser + printer).
  * **Checker** requires unsafe context to call **extern** fns:
    * **`DFI0003`** “Extern call in safe context” with help/suggestion to wrap in `unsafe:`.
  * Reserved/added diag codes: **`DFI0001`** (invalid `@extern` args), **`DFI0002`** (`@extern` must be on `pub def`) — enforcement can ship later.
* **Extern functions**:
  * **Decorator** `@extern("C"[,"lib"])` on **`pub def`**.
  * **Resolver** records per-overload extern metadata `{Extern:true, ABI:"C", Link:optional}`.
  * **Lowerer** skips HIR bodies for extern prototypes.
  * **Backend (LLVM)** emits **exactly one `declare`** per referenced extern symbol, before `define`s; no duplicates.
* **Imports & re-exports**:
  * **Package re-exports** supported: `math/__mod.desi` can `from math.add import add`, and consumers can `from math import add`.
    * Resolver merges re-exports into the package’s **typed** export surface (overloads & extern metadata copied through).
  * (Relative imports remain out-of-scope; absolute imports only.)
* **Library stubs (import-driven FFI)**:
  * `compiler/lib/sqlite3/__mod.desi`, `compiler/lib/crypto/__mod.desi` provide extern prototypes for demos.
* **Examples / tests**:
  * `examples/17_m9_ffi_libm.desi` — `sin` via `@extern`, guarded by `unsafe:`.
  * `examples/18_m9_ffi_sqlite.desi` — import-driven sqlite demo (compile-only).
  * Backend tests:
    * `ffi_decl_test.go` (single `declare` per extern),
    * `types_lower_test.go` (Tier-0 mappings),
  * Resolver test: `reexports_test.go`.
  * Checker test: `ffi_unsafe_calls_test.go` (DFI0003).

**Acceptance**

* `go build ./... && go test ./...` **green**.
* Externs in user code must be called inside `unsafe:`; violations produce **DFI0003**.
* IR contains **one `declare` per extern symbol** that is actually referenced.
* `examples/17_m9_ffi_libm.desi` and `examples/18_m9_ffi_sqlite.desi` **parse, resolve, type-check, and emit IR** (link/run is out-of-scope for Tier-0).
* **Deferred to M10+**: manifest-driven linking (tooling), relative imports, decorator named args like `link="…"`, `symbol="…"`.

---

### M10 — Collections & Prelude v1

**Scope**

* std prelude: `print`, `len`, `bool`, `str`, `map`, `filter`, `range`.
* Built-in collection ops (`push`, `set`, membership `in`).
* Comprehensions compiled efficiently.

**Acceptance**

* Collections & comprehension examples pass and are fast (microbench).

---

### M11 — Formatter v1 (AST pretty-printer)

**Scope**

* Idempotent; no options initially; respects docstrings/decorators/import grouping.
* Enforce single-line/multiline rules, spacing, indentation.

**Acceptance**

* Golden format tests: input → expected output → reformat = no diff.

---

### M12 — Diagnostics Polish & Tooling

**Scope**

* Caret/underline renderer with color; JSON diagnostics (`--error-format=json`) for IDEs.
* Snapshot tests for error rendering.
* **Developer ergonomics:** simple `Makefile` (`make build/test/tokens/demo-layout`; later `fmt`, `vet`, `lint`).

**Acceptance**

* Matches Rust-like output style; IDE plugin PoC consumes JSON.
* `make test` green; helper targets work locally and in CI.

---

### M13 — Packaging & Std Growth

**Scope**

* `desi init`, `desi build`, `desi run`, `desi test`.
* Expand std modules (`string`, `task`, `fs`, `time`), dataclass decorator, etc.

**Acceptance**

* Example projects build/run; documentation synced.

---

## Cross-cutting practices

* **Testing:** unit tests per package; golden tests for diagnostics & formatter; integration smoke tests for codegen.
* **Performance:** microbenchmarks for tight loops, pipelines, comprehensions; keep codegen O2-friendly.
* **Documentation:** keep `docs/grammar.ebnf` and guides canonical and updated.
* **Style:** enforced by `desifmt` (no options).
