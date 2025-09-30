# Desi — M-Series Milestones (status + context)

Legend: ✅ done · 🚧 in progress · ⏳ not started

## Snapshot

| ID  | Milestone                                | Status | Notes / Artifacts                                |
|-----|------------------------------------------|:------:|--------------------------------------------------|
| M1  | Single-line function defs                |   ✅    | `examples/singleline_def.desi`                   |
| M2  | Single-line `if`                         |   ✅    | `examples/singleline_if*.desi`                   |
| M3  | Compound assignments (`+= −= *= /=`)     |   ✅    | `examples/plus_assign.desi`                      |
| M4  | Dotted call exprs (`io.println(x)`)      |   ✅    | `examples/str_api_demo.desi`                     |
| M5  | String literals + escapes                |   ✅    | used across examples                             |
| M6  | Type aliases                             |   ✅    | `examples/type_alias.desi`                       |
| M7  | Structs                                  |   ✅    | see section below                                |
| M8  | Enums / tagged unions                    |   ✅    | see section below                                |
| M9  | Import hygiene                           |   ✅    | module aliases, from-imports, diagnostics        |
| M10 | Public/exported decls                    |   ✅    | Phase A+B + `pub enum`/`pub type`                |
| M11 | `async` / `await` (minimal)              |   ⏳    | grammar + state-machine lowering + tiny executor |
| M12 | Channels & `spawn`                       |   ⏳    | MPSC channel + `spawn` on single-thread executor |
| M13 | C-ABI module boundary                    |   ⏳    | `.a/.so` + `.dmi`                                |
| M14 | Spans & pretty errors everywhere         |   🚧   | many wired; broadening coverage                  |
| M15 | Watch mode (`desic dev`)                 |   ⏳    | —                                                |
| M16 | Docstrings & `desic doc`                 |   ⏳    | module/func/struct/enum doc + Markdown output    |
| M17 | Multiline strings (`"""..."""`)          |   ⏳    | nicer docs/examples                              |
| M18 | Borrowing surface (`ref T` / `inout T`)  |   ⏳    | front-end borrow checker, move semantics         |
| M19 | ARC runtime shim (C backend)             |   ⏳    | retain/release for owned aggregates              |
| M20 | LLVM backend (experimental)              |   ⏳    | SSA lowering, sanitizer integration              |
| M21 | Toolchains (“virtual env”)               |   ⏳    | `desi.toml`, `desi use`, per-project pinning     |
| M22 | Package manager                          |   ⏳    | `desi pkg add name@^ver` + lockfile              |
| M23 | Stdlib growth (prelude/io/fs/json/net/…) |   ⏳    | versioned with toolchain                         |

---

## M7 — Structs (✅ Completed)

### Implemented

* **Parser**
  * Struct decls:
    `struct Name:\n  first: str\n  last: str`
  * Struct literals (designated): `User{ id: 1, name: "X" }`
  * Dotted LHS assignment (nested OK): `u.id := 42`, `u.name.first := "Ada"`
* **Checker**
  * `Info.Structs` (field map).
  * Nested field access typing; base var marked read.
  * Field-assignment validation along full chain.
* **Codegen (C)**
  * `typedef struct { ... } Name;`
  * Locals of struct type; nested field assign.
  * Designated initializers, incl. nested.
  * Assignments use **LHS expressions** (ident or `a.b.c`); single assigns are temp-free. Parallel assigns still use temps to preserve order.

### Examples/Tests

`examples/struct_basic.desi`, `examples/struct_field_typecheck*.desi`,
`examples/struct_phase_three.desi`, `examples/m7_structs_funcs.desi`,
`examples/m7_structs_nested_assign.desi`, legacy `examples/parallel_demo.desi`

### Follow-ups

* Move “undeclared assign” warnings from C emitter → **checker** (`docs/goals/codegen-cleanup.md`).

---

## M8 — Enums / Tagged Unions (✅ Completed)

### Implemented

**Parser**

* `enum` declarations (`Variant: payloadType`), `none` ⇒ no payload.
* `match` statement:
  * Scrutinee may be **any expression** (identifier, function call, constructor literal).
  * Wildcard arm `_:` for default.
  * Ignore binder `_` inside payloads.
  * Wildcard with payload is a parser error.

**Checker**

* `Info.Enums` collected.
* Constructor checks:
  * Arity and payload type validation with **detailed diagnostics** (e.g., `wrong payload type for Result.Ok: expected int, got str`).
* `match` rules:
  * Scrutinee must be enum (or Unknown while editing).
  * Duplicate/unknown variants error.
  * Binder typed from payload; `_` binder ignored.
  * Exhaustiveness warning **names the enum** and lists missing variants.
* Function returns:
  * **Implicit tail-expression return** accepted when type-compatible.

**Codegen (C)**

* Tagged union lowering:

```c
#define Result_Ok 0
#define Result_Err 1
typedef struct { int tag; union { int Ok; const char* Err; } as; } Result;
```

* Constructors via designated inits.
* `match` → `switch (__scrut.tag)` with per-variant binder extraction.
* Scrutinee evaluated once into `__scrut`.
* `io.println` joins multiple args with spaces.
* Tail expression lowered as `return <expr>;`.

**Emitter cleanup**

* Removed legacy `AssignStmt.Names` path.

### Examples/Tests

* Basic: `examples/m8_enum_result.desi`, `examples/m8_enum_maybe.desi`
* Match features: `examples/m8_match_wildcard.desi`, `examples/m8_match_ignore_binder.desi`
* Non-ident scrutinee: `examples/m8_match_call_scrutinee.desi`, `examples/m8_match_ctor_literal_scrutinee.desi`
* Diagnostics & println: `examples/m8_ctor_payload_type_error.desi`, `examples/m8_match_non_exhaustive.desi`, `examples/println_spacing.desi`

**Docs**

* `docs/spec/enums.md` finalized with wildcard, ignore binder, non-ident scrutinee.
* `docs/spec/grammar_enums.ebnf` updated.

---

## M9 — Import hygiene (✅ Completed)

### Implemented

**Parser**

* Plain module aliasing: `import util.math as m`.
* From-imports with optional aliasing and spans:

  * `from util.math import add`
  * `from util.math import add as addition, sub`
* Friendlier identifier diagnostics when a keyword is used where an identifier is required (alias/name):

  * `DPE0002: keyword "def" cannot be used as an identifier for alias …`
* `ast` JSON encode/decode updated to include `FromImportDecl` + `ImportItem` with spans.

**Loader / Resolver**

* Cross-platform module resolution against:

  * current project directory, then
  * each ancestor `compiler/lib` directory (OS-parity via `filepath` + symlink-aware comparisons).
* Diagnostics (typed, with codes):

  * `DME0001 import cycle` with a/b/c chain.
  * `DME0002 cannot find module "X"` with “looked for:” attempted paths.
  * `DMW0001 duplicate import` (per-file, de-duplicated by module path).
* Merge order preserved: entry module first, then dependencies.

**Checker**

* From-import alias map threads into function bodies:

  * `addition(1,2)` resolves to `util.math.add` signature.
* Module alias calls: `m.add(x)` type-checked against actual function signature.
* Shadowing rules: local bindings/params **override** module aliases:

  * `let m = Point{...}; m.x` is a struct field access, not a module reference.
* Diagnostics:

  * Bare module alias used as a value: `error: module alias "m" (from "util.math") is not a value; use m.<symbol>`.
  * Unknown symbol under a module alias: `error: unknown symbol "nope" in module alias "m"`.
  * Duplicate `from … import` aliases on the same line flagged clearly.
  * Undefined names carry spans when available.

**Codegen (C)**

* Expression statements that don’t produce a value are lowered safely (e.g., `(void)(m.x);`) to avoid unused-value warnings.
* No special casing otherwise—module aliasing is purely a front-end concern.

### Examples/Tests

* `examples/m9_import_alias.desi`
* `examples/m9_module_alias_as_value.desi`
* `examples/m9_module_alias_call.desi`
* `examples/m9_module_unknows_symbol.desi`
* `examples/m9_local_var_shadowing.desi`
* `examples/m9_alias_collision_module_vs_from.desi`
* `examples/m9_alias_collision_from_same_line.desi`
* `examples/m9_alias_reserved_name.desi`

---

## M10 — Public/exported decls (✅ Completed)

**Why:** Desi needs compile-time encapsulation for API clarity, optimization, and concurrency safety.

### Phase A — syntax + plumbing (✅)

* **Lexer:** `pub` token.
* **Parser:** optional `pub` before top-level `def`, `struct`, and **const** (`let` at top level). Parser accepts `pub let mut` so the checker can flag it.
* **AST:** `Pub: bool` on `FuncDecl`, `StructDecl`, `ConstDecl`.
* **Checker:** collects `Public` tables and tracks top-level constants for intra-module typing.
* **AST JSON:** round-trips `pub` on `FuncDecl`, `StructDecl`, `ConstDecl`.

### Phase B — visibility enforcement (✅)

* **Rules enforced**:

  * **From-imports**: `from X import Y` must import **public** symbols. Non-public import sites produce `DTE0010` with a span on the item.
  * **Module-alias calls**: `m.f(...)` require `f` to be public, else `DTE0010`.
  * **Module-alias constants**: `m.CONST` is typed to the const’s kind and requires the const to be public, else `DTE0010`.
  * **Unqualified cross-module calls**: Using `foo()` from another module **without** a `from` alias or `m.foo` is rejected with `DTE0010`.
  * **Public const rules**: `pub let mut` → `DTE0012`. `pub let X = <non-literal>` → `DTE0011`.

* **Checker wiring**:

  * Eager check on `from … import` items (span on each item).
  * Use-site checks for `m.symbol`, unqualified calls, and alias lookups.
  * Identifiers that are from-imported constants type as their literal kind.

* **Codegen (C)**:

  * **Enum constructors** lower directly: `Mode.On` → `(Mode){ .tag = Mode_On }`.
  * Top-level constants are safely handled during emission so no undefined C names leak from other modules.

* **Examples/Tests**:

  * **Negative**: `examples/m10_vis/mod/main.desi` (unqualified call to private `util.math.add` → `DTE0010`).
  * **Positive**:

    * `examples/m10_vis/mod_pub/main_alias.desi` (`import util.math as m` → `m.add` with `pub def add`).
    * `examples/m10_vis/mod_pub/main_from.desi` (`from util.math import add` with `pub def add`).
  * **Types & Enums visibility**:

    * `examples/m10_pub_type_enum/mod/main_ok.desi`
    * `examples/m10_pub_type_enum/mod/main_bad.desi`

### Nice-to-haves (delivered as part of M10 ✅)

* **`pub enum`** and **`pub type`**:

  * Parser/AST/JSON support (`Pub: bool` on `EnumDecl` and `TypeDecl`).
  * Checker visibility enforcement for types/enums in `from` imports and module-alias contexts.
  * Codegen fix for **enum constructors** (see above).

### Out of scope for M10 (tracked for later)

* **Per-module symbol tables** (avoid global-map collisions as the language grows).
* **C namespacing** once we ship a multi-module C-ABI boundary.

---

## M14 — Diagnostics (🚧 Ongoing)

* **Baseline ✅**: JSON codes registry + Rust-style renderer.
* **Typed checker errors carry spans** where available (e.g., undefined name; M10 not-public errors).
* Parser/bridge still accept legacy lines; expanding structured spans across more constructs.
* Next: broaden span coverage in checker/codegen; add secondary notes/suggestions for common cases.
