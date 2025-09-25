# Desi — M-Series Milestones (status + context)

Legend: ✅ done · 🚧 in progress · ⏳ not started

## Snapshot

| ID  | Milestone                            | Status | Notes / Artifacts                                        |
|-----|--------------------------------------|:------:|----------------------------------------------------------|
| M1  | Single-line function defs            |   ✅    | `examples/singleline_def.desi`                           |
| M2  | Single-line `if`                     |   ✅    | `examples/singleline_if*.desi`                           |
| M3  | Compound assignments (`+= −= *= /=`) |   ✅    | `examples/plus_assign.desi`                              |
| M4  | Dotted call exprs (`io.println(x)`)  |   ✅    | `examples/str_api_demo.desi`                             |
| M5  | String literals + escapes            |   ✅    | used across examples                                     |
| M6  | Type aliases                         |   ✅    | `examples/type_alias.desi`                               |
| M7  | Structs                              |   ✅    | see section below                                        |
| M8  | Enums / tagged unions                |   🚧   | **Code ✅ (this pass); docs pending** — see section below |
| M9  | Import hygiene                       |   ⏳    | cycles, OS parity                                        |
| M10 | Public/exported decls                |   ⏳    | `pub def`, `pub struct`                                  |
| M11 | `async` / `await` (minimal)          |   ⏳    | —                                                        |
| M12 | Channels & `spawn`                   |   ⏳    | —                                                        |
| M13 | C-ABI module boundary                |   ⏳    | `.a/.so` + `.dmi`                                        |
| M14 | Spans & pretty errors everywhere     |   🚧   | lexer/parser many; full coverage pending                 |
| M15 | Watch mode (`desic dev`)             |   ⏳    | —                                                        |

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

## M8 — Enums / Tagged Unions (🚧 Code ✅; docs pending)

### Syntax (Stage-1)

```desi
enum Result:
  Ok: int
  Err: str

enum MaybeStr:
  Some: str
  None: none
````

* Constructors are qualified calls: `Result.Ok(200)`, `Result.Err("boom")`, `MaybeStr.None()`.
* `none|void|""` payload is treated as **no payload**.

### Implemented (this pass)

**Parser**

* `enum` declarations (`Variant: payloadType`), `none` ⇒ no payload.
* `match` statement now supports **non-identifier scrutinee**:

  * `match r: ...`
  * `match make_result(): ...`
  * `match Result.Ok(1): ...`
* **Wildcard arm** `_:` and **ignore binder** `_`:

  * `Err(_): ...`
  * wildcard may **not** have a payload (parser error).

**Checker**

* Collect `Info.Enums` (Variant → payload textual type).
* Constructor checking:

  * Arity validation.
  * Payload type validation with improved diagnostics:

    * `wrong payload type for Result.Ok: expected int, got str`
    * `Result.Ok expects 1 arg(s), got N`
* `match` rules:

  * Scrutinee must be enum (or Unknown while editing).
  * Unknown/duplicate variants → error.
  * Binder typed from variant payload; `_` allowed/ignored.
  * **Exhaustiveness warning** lists missing variants and **names the enum**:

    * `non-exhaustive match on enum Result: missing Ok, Err`
* Function analysis:

  * **Implicit tail-expression return** accepted (suppresses DW0006) when the last statement is an `ExprStmt` whose type unifies with the function’s return type.

**Codegen (C)**

* Tagged-union lowering:

  ```c
  #define Result_Ok 0
  #define Result_Err 1
  typedef struct { int tag; union { int Ok; const char* Err; } as; } Result;
  ```
* Constructors lower via designated inits.
* `match`:

  * Scrutinee expression is evaluated **once** into `__scrut`, then we `switch (__scrut.tag)`.
  * Payload binder (if not `_`) is declared with the correct C type.
* `io.println` joins multiple arguments with spaces:

  * `io.println("a", "b", 1)` → `printf("%s %s %d\n", ...)`.
* **Implicit tail return** lowering:

  * If the last statement is an `ExprStmt` in a non-void function, it becomes `return <expr>;`.

**Emitter cleanup**

* Removed legacy `AssignStmt.Names` path from **emitter** and **checker**; assignments now use LHS expressions exclusively.

### Examples/Tests

* Constructors & basics: `examples/m8_enum_result.desi`, `examples/m8_enum_maybe.desi`
* Match features:

  * Wildcard: `examples/m8_match_wildcard.desi`
  * Ignore binder: `examples/m8_match_ignore_binder.desi`
* New scrutinee support:

  * `examples/m8_match_call_scrutinee.desi`
  * `examples/m8_match_ctor_literal_scrutinee.desi`
* Diagnostics & println:

  * `examples/m8_ctor_payload_type_error.desi`
  * `examples/m8_match_non_exhaustive.desi`
  * `examples/println_spacing.desi`

### Open items (to finish M8)

* **Docs** (Task 4, pending in this pass):

  * `docs/spec/enums.md`: add wildcard / ignore-binder sections, pitfalls.
  * `docs/spec/grammar_enums.ebnf`: include non-identifier scrutinee.
* (Nice-to-have) Point missing-variant notes at the enum decl site (spans/notes).

---

## M14 — Diagnostics (🚧 Ongoing)

* JSON codes registry + Rust-style renderer (baseline ✅).
* Bridge parses legacy lexer lines; plan to transition to structured `DIAG` rows.
* To do: propagate spans from parser/checker everywhere; pretty notes/suggestions.

