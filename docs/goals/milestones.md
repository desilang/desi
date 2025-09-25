# Desi — M-Series Milestones (status + context)

Legend: ✅ done · 🚧 in progress · ⏳ not started

## Snapshot

| ID  | Milestone                            | Status | Notes / Artifacts                        |
| --- | ------------------------------------ | :----: | ---------------------------------------- |
| M1  | Single-line function defs            |    ✅   | `examples/singleline_def.desi`           |
| M2  | Single-line `if`                     |    ✅   | `examples/singleline_if*.desi`           |
| M3  | Compound assignments (`+= −= *= /=`) |    ✅   | `examples/plus_assign.desi`              |
| M4  | Dotted call exprs (`io.println(x)`)  |    ✅   | `examples/str_api_demo.desi`             |
| M5  | String literals + escapes            |    ✅   | used across examples                     |
| M6  | Type aliases                         |    ✅   | `examples/type_alias.desi`               |
| M7  | Structs                              |    ✅   | see section below                        |
| M8  | Enums / tagged unions                |   🚧   | see section below                        |
| M9  | Import hygiene                       |    ⏳   | cycles, OS parity                        |
| M10 | Public/exported decls                |    ⏳   | `pub def`, `pub struct`                  |
| M11 | `async` / `await` (minimal)          |    ⏳   | —                                        |
| M12 | Channels & `spawn`                   |    ⏳   | —                                        |
| M13 | C-ABI module boundary                |    ⏳   | `.a/.so` + `.dmi`                        |
| M14 | Spans & pretty errors everywhere     |   🚧   | lexer/parser many; full coverage pending |
| M15 | Watch mode (`desic dev`)             |    ⏳   | —                                        |

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
* Remove legacy `AssignStmt.Names` fallback in emitter (post-M8).

---

## M8 — Enums / Tagged Unions (🚧 In progress)

### What enums look like (current Stage-1 syntax)

```desi
enum Result:
  Ok: int
  Err: str

enum MaybeStr:
  Some: str
  None: none
```

* Constructors are qualified calls: `Result.Ok(200)`, `Result.Err("boom")`, `MaybeStr.None()`.
* `none|void|""` payload is treated as **no payload**.

### Implemented so far

**Parser**

* `enum` declarations (`Variant: payloadType`), `none` means no payload.
* `match` statement (Stage-1 scrutinee is an identifier):

  ```
  match r:
    Ok(x): ...
    Err(e): ...
  ```
* **Wildcard arm** `_:` (default case) supported.
* **Ignore binder** `_` allowed in payload position: `Err(_): ...`
* **Validation:** wildcard may **not** take a payload → parser error `"wildcard '_' cannot have a payload"`.

**Checker**

* Collect `Info.Enums` (Variant → payload textual type).
* Constructor calls typed by payload:

  * Arity check (no/one arg depending on variant).
  * Type check against payload type.
* `match` rules:

  * Scrutinee must be enum (or Unknown → still check bodies).
  * Unknown variant → error.
  * Duplicate arm for same variant → error.
  * Binder (e.g., `x` in `Ok(x)`) is typed to the variant payload; `_` binder is permitted and ignored.
  * **Exhaustiveness:** if no wildcard arm, warn and list missing variants.
  * With wildcard arm `_:` present, exhaustiveness warning is suppressed.

**Codegen (C)**

* Tagged union layout per enum:

  ```c
  #define Result_Ok 0
  #define Result_Err 1
  typedef struct { int tag; union { int Ok; const char* Err; } as; } Result;
  ```
* Constructors lower to designated initializers:

  * No payload: `(Enum){ .tag = Enum_Variant }`
  * With payload: `(Enum){ .tag = Enum_Variant, .as.Variant = <expr> }`
* `match` lowers to:

  ```c
  Enum __scrut = r;
  switch (__scrut.tag) {
    case Result_Ok: { int x = __scrut.as.Ok; ...; break; }
    case Result_Err: { const char* e = __scrut.as.Err; ...; break; }
    default: { ... } // from wildcard arm
  }
  ```
* `printf` format selection in `match` bodies respects inferred payload type (avoids `%d`/`%s` mismatch warnings).

### Examples/Tests

* Constructors & basic usage: `examples/m8_enum_result.desi`, `examples/m8_enum_maybe.desi`
* New features:

  * Wildcard arm: `examples/m8_match_wildcard.desi`
  * Ignore binder: `examples/m8_match_ignore_binder.desi`

### Open items / Next steps (M8)

1. **Parser**

  * Allow non-identifier scrutinee: `match compute(): ...` (currently ident-only).
  * Optional: support `match e` on a single line with inline blocks (later).
2. **Checker**

  * Point missing-variant diagnostics at the enum declaration (nice-to-have).
  * Enrich notes for wrong-payload types (tell which variant & expected type).
3. **Codegen**

  * Remove legacy `AssignStmt.Names` path once tests are migrated.
  * Consider emitting `enum` + `struct` tags as `enum { ... }` for readability (cosmetic).
4. **Spec/Docs**

  * Finalize `docs/spec/enums.md` & `docs/spec/grammar_enums.ebnf` (drafts added).
  * Document `_:` wildcard & `_` ignore binder with gotchas (no payload on `_`).
5. **Future (post-M8)**

  * Pattern guards (`Ok(x) if x > 0:`).
  * Multi-field payloads (tuple/struct-like) and destructuring.
  * Generic Result/Option once generics land.

---

## M14 — Diagnostics (🚧 Ongoing)

* JSON codes registry + Rust-style renderer (baseline ✅).
* Bridge parses legacy lexer lines; plan to transition to structured `DIAG` rows.
* To do: propagate spans from parser/checker everywhere; pretty notes/suggestions.

---

## Suggested next actions

1. **M8 polish**: enable non-ident scrutinee; add richer checker messages; finish docs/examples.
2. **Emitter cleanup**: delete legacy `Names` path (make tests use LHS exprs).
3. **M14**: start emitting structured `DIAG` from parser/checker for a couple of cases (pilot).

