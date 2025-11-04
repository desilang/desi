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

### M6 — Borrow Checker (Function-local, Phase 1, ✅ DONE)

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

### M8 — Async & Futures

**Scope**

* Lower async funcs to state machines; `await`, `join`, `with_timeout`, `select`.
* Enforce borrow rules at suspension points.
* **Async lambda** expression form (expression-only body): `async lambda x: await f(x)`.

**Acceptance**

* Demos run; async lambda covered by tests.
* Reject `inout` across `await` with clear diagnostics.

---

### M9 — FFI v1 (C ABI)

**Scope**

* Manifest-driven linking (`desi.toml`).
* `@extern("C", link="…")` decorators.
* `cptr[T]`, `usize/isize`, null checks, minimal `unsafe` blocks.
* Examples: `libm` (`sin/cos`), `sqlite3`, minimal OpenSSL SHA256`.

**Acceptance**

* `unit_circle` & sqlite demos run.
* Type mismatches produce `DESI-FFI-*` with helpful help text.

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
