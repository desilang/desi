# Desi — M-Series Milestones (status + context)

Legend: ✅ done · 🚧 in progress · ⏳ not started

## Snapshot

| ID  | Milestone                            | Status | Notes / Artifacts                         |
|-----|--------------------------------------|:------:|-------------------------------------------|
| M1  | Single-line function defs            |   ✅    | `examples/singleline_def.desi`            |
| M2  | Single-line `if`                     |   ✅    | `examples/singleline_if*.desi`            |
| M3  | Compound assignments (`+= −= *= /=`) |   ✅    | `examples/plus_assign.desi`               |
| M4  | Dotted call exprs (`io.println(x)`)  |   ✅    | `examples/str_api_demo.desi`              |
| M5  | String literals + escapes            |   ✅    | used across examples                      |
| M6  | Type aliases                         |   ✅    | `examples/type_alias.desi`                |
| M7  | Structs                              |   ✅    | see section below                         |
| M8  | Enums / tagged unions                |   ✅    | see section below                         |
| M9  | Import hygiene                       |   ✅    | module aliases, from-imports, diagnostics |
| M10 | Public/exported decls                |   ⏳    | `pub def`, `pub struct`                   |
| M11 | `async` / `await` (minimal)          |   ⏳    | —                                         |
| M12 | Channels & `spawn`                   |   ⏳    | —                                         |
| M13 | C-ABI module boundary                |   ⏳    | `.a/.so` + `.dmi`                         |
| M14 | Spans & pretty errors everywhere     |   🚧   | lexer/parser many; full coverage pending  |
| M15 | Watch mode (`desic dev`)             |   ⏳    | —                                         |

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
  * Unknown symbol under a module alias: `error: unknown symbol "nope" in module alias "m" (from "util.math")`.
  * Duplicate `from … import` aliases on the same line flagged clearly.
  * Undefined names carry spans when available.

**Codegen (C)**

* Expression statements that don’t produce a value are lowered safely (e.g., `(void)(m.x);`) to avoid unused-value warnings.
* No special casing otherwise—module aliasing is purely a front-end concern.

### Examples/Tests

* `examples/m9_import_alias.desi`
* `examples/m9_module_alias_as_value.desi`
* `examples/m9_module_unknows_symbol.desi`
* `examples/m9_local_var_shadowing.desi`
* `examples/m9_alias_collision_module_vs_from.desi`
* `examples/m9_alias_collision_from_same_line.desi`
* `examples/m9_alias_reserved_name.desi`

---

## M14 — Diagnostics (🚧 Ongoing)

* **Baseline ✅**: JSON codes registry + Rust-style renderer.
* **Typed checker errors carry spans** where available (e.g., undefined name).
* Bridge still parses legacy lexer lines; we’ll transition to structured `DIAG` rows.
* Next: propagate spans from parser/checker broadly; add secondary notes/suggestions for common cases.

