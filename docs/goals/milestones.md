# M-Series Milestones — Live Tracker

Legend: ✅ done · 🚧 in progress · ⏳ not started

> Source of truth for milestone status across parser/checker/codegen, with pointers to examples/tests.

## Snapshot

| ID  | Milestone                               | Status | Notes / Artifacts |
|-----|-----------------------------------------|:-----:|-------------------|
| M1  | Single-line function defs               | ✅    | `examples/singleline_def.desi` |
| M2  | Single-line `if`                        | ✅    | `examples/singleline_if*.desi` |
| M3  | Compound assignments (`+= −= *= /=`)    | ✅    | `examples/plus_assign.desi` |
| M4  | Dotted call exprs (`io.println(x)`)     | ✅    | `examples/str_api_demo.desi` |
| M5  | String literals + escapes               | ✅    | used across examples |
| M6  | Type aliases                            | ✅    | `examples/type_alias.desi` |
| M7  | Structs                                 | ✅    | see details below |
| M8  | Enums / tagged unions                   | 🚧    | see details below |
| M9  | Import hygiene                          | ⏳    | cycles, OS parity |
| M10 | Public/exported decls                   | ⏳    | `pub def`, `pub struct` |
| M11 | `async` / `await` (minimal)             | ⏳    | — |
| M12 | Channels & `spawn`                      | ⏳    | — |
| M13 | C-ABI module boundary                   | ⏳    | `.a/.so` + `.dmi` |
| M14 | Spans & pretty errors everywhere        | 🚧    | parser/lexer many; full coverage pending |
| M15 | Watch mode (`desic dev`)                | ⏳    | — |

---

## M7 — Structs (Completed)

### Implemented
- **Parser**
  - Struct decls (`struct Name:\n  field: type`).
  - Struct **literals** `User{ id: 1, name: "X" }`.
  - **Dotted LHS assignment** `u.id := 42`, `u.name.first := "Ada"`.
- **Checker**
  - `Info.Structs` with field maps.
  - Nested field access typing + base-var read.
  - Field-assignment validation along the chain.
- **Codegen (C)**
  - `typedef struct { ... } Name;`.
  - Locals of struct type; nested field assign.
  - Designated initializers; nested inits.
  - Assignments use **LHS expr**; single assigns are temp-free.

### Examples/Tests
`examples/struct_basic.desi`, `examples/struct_field_typecheck*.desi`,
`examples/struct_phase_three.desi`, `examples/m7_structs_funcs.desi`,
`examples/m7_structs_nested_assign.desi`, `examples/parallel_demo.desi`

### Follow-ups
- Move undeclared-assign warnings to **checker** (see `docs/goals/codegen-cleanup.md`).
- Remove legacy `Names` fallback in emitter (post-M8).

---

## M8 — Enums / Tagged Unions (In Progress)

### Implemented
- **Parser**
  - Enum decls:
    ```desi
    enum Result:
      Ok: int
      Err: str
    ```
  - `match` statement:
    ```desi
    match r:
      Ok(x): ...
      Err(e): ...
    ```
- **Checker**
  - Collect `Info.Enums` (variant → payload type).
  - Constructor calls typed by variant payload (arity and type).
  - `match`:
    - Scrutinee must be enum.
    - Unknown variant errors.
    - Duplicate-arm error.
    - Binder typed to payload.
    - Non-exhaustive **warning** listing missing variants.
- **Codegen (C)**
  - C tagged union: `tag` + `union { ... } as;` and `#define Enum_Variant`.
  - Constructors lower to designated initializers.
  - `match` lowers to `switch(tag)` with payload extraction.

### Examples/Tests
- `examples/m8_enum_result.desi`
- `examples/m8_enum_maybe.desi`

### Next
- Wildcard/“default” arm (`_:`) → optional Stage-1 sugar (silences non-exhaustive).
- Better diagnostics: point to missing variants in enum decl.
- (Optional) Allow ignoring payload via `_` binder.

---

## How to update

- Keep the table concise; put details under the milestone section.
- When you land work, add the example(s)/test(s) and flip the status.
- Cross-link deeper design docs (RFCS/spec) rather than duplicating content.
