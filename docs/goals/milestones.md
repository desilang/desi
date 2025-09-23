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
| M8  | Enums / tagged unions                   | ⏳    | — |
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
  - Struct **literals** with designated init: `User{ id: 1, name: "X" }`.
  - **Dotted LHS assignment**: `u.id := 42`, `u.name.first := "Ada"`.
- **Checker**
  - `Info.Structs` with field maps.
  - Nested field access typing (`a.b.c`) + mark base var as read.
  - Field-assignment validation along the full chain.
- **Codegen (C)**
  - `typedef struct { ... } Name;` emission.
  - Locals of struct type; nested field assignment.
  - Struct literals as designated initializers:
    - `(User){ .id = 1, .name = "X" }`
    - Nested: `(User){ .name = (Name){ .first = "x" } }`
  - **Temps pruned** for single assignments (kept only for parallel assigns).

### Examples/Tests
- `examples/struct_basic.desi`
- `examples/struct_field_typecheck.desi`
- `examples/struct_field_typecheck_bad.desi`
- `examples/struct_phase_three.desi`
- `examples/m7_structs_funcs.desi`
- `examples/m7_structs_nested_assign.desi`

### Follow-ups (tracked)
- Move “undeclared assign” warnings from C emitter → **checker** (see `docs/goals/codegen-cleanup.md`).
- Switch C emitter to use `AssignStmt.LHS` (Expr) fully; remove legacy `Names` path.
- Consider struct pass/return by value (decide in M7 postscript or push to M8).

---

## How to update

- Keep the table concise; put details under the milestone section.
- When you land work, add the example(s)/test(s) and flip the status.
- Cross-link deeper design docs (RFCS/spec) rather than duplicating content.

