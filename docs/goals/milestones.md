# Desi — M-Series Milestones (status + context)

Legend: ✅ done · 🚧 in progress · ⏳ not started

## Snapshot

| ID  | Milestone                            | Status | Notes / Artifacts                        |
|-----|--------------------------------------|:------:|------------------------------------------|
| M1  | Single-line function defs            |   ✅    | `examples/singleline_def.desi`           |
| M2  | Single-line `if`                     |   ✅    | `examples/singleline_if*.desi`           |
| M3  | Compound assignments (`+= −= *= /=`) |   ✅    | `examples/plus_assign.desi`              |
| M4  | Dotted call exprs (`io.println(x)`)  |   ✅    | `examples/str_api_demo.desi`             |
| M5  | String literals + escapes            |   ✅    | used across examples                     |
| M6  | Type aliases                         |   ✅    | `examples/type_alias.desi`               |
| M7  | Structs                              |   ✅    | see section below                        |
| M8  | Enums / tagged unions                |   ✅    | see section below                        |
| M9  | Import hygiene                       |   ⏳    | cycles, OS parity                        |
| M10 | Public/exported decls                |   ⏳    | `pub def`, `pub struct`                  |
| M11 | `async` / `await` (minimal)          |   ⏳    | —                                        |
| M12 | Channels & `spawn`                   |   ⏳    | —                                        |
| M13 | C-ABI module boundary                |   ⏳    | `.a/.so` + `.dmi`                        |
| M14 | Spans & pretty errors everywhere     |   🚧   | lexer/parser many; full coverage pending |
| M15 | Watch mode (`desic dev`)             |   ⏳    | —                                        |

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
  * Arity and payload type validation with detailed diagnostics (`Result.Ok expects int, got str`).
* `match` rules:
  * Scrutinee must be enum.
  * Duplicate/unknown variants error.
  * Binder typed from payload; `_` binder ignored.
  * Exhaustiveness warning names the enum and lists missing variants.
* Function return analysis:
  * Accepts **implicit tail expression return** when type-compatible.

**Codegen (C)**

* Tagged union lowering:
```c
  #define Result_Ok 0
  #define Result_Err 1
  typedef struct { int tag; union { int Ok; const char* Err; } as; } Result;

```

* Constructors lower via designated inits.
* `match` lowers to `switch (__scrut.tag)` with binder extraction.
* Scrutinee evaluated once into `__scrut`.
* `io.println` joins multiple args with spaces.
* Implicit tail expression → lowered as `return <expr>;`.

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

## M14 — Diagnostics (🚧 Ongoing)

* JSON codes registry + Rust-style renderer (baseline ✅).
* Bridge parses legacy lexer lines; plan to transition to structured `DIAG` rows.
* To do: propagate spans from parser/checker everywhere; pretty notes/suggestions.
