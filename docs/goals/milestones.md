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
  - **Assignments use LHS expr** (ident or nested field). Legacy `Names` path retained for old tests.
  - **Single assigns** emit directly (no temps). **Parallel assigns** still use temps to preserve eval order.

### Examples/Tests
- `examples/struct_basic.desi`
- `examples/struct_field_typecheck.desi`
- `examples/struct_field_typecheck_bad.desi`
- `examples/struct_phase_three.desi`
- `examples/m7_structs_funcs.desi`
- `examples/m7_structs_nested_assign.desi`
- (legacy parallel) `examples/parallel_demo.desi`

### Follow-ups (tracked)
- Move “undeclared assign” warnings from C emitter → **checker** (see `docs/goals/codegen-cleanup.md`).
- LHS in emitter: **✅ A1 done**; remove legacy `Names` fallback in M8.
- Consider struct pass/return by value (decide in M7 postscript or push to M8).

---

## M8 — Enums / Tagged Unions (In Progress)

### Syntax (Phase-1)
```desi
enum Result:
  Ok: int
  Err: str

enum MaybeStr:
  Some: str
  None: none  # zero-payload variant (maps to void/unit)
```

* Construction: `Result.Ok(42)`, `Result.Err("boom")`, `MaybeStr.Some("x")`, `MaybeStr.None()`.
* Accessors/helpers auto-generated in Phase-1:

  * `Result.is_Ok(x) -> bool`, `Result.unwrap_Ok(x) -> int`
  * `MaybeStr.is_Some(x) -> bool`, `MaybeStr.unwrap_Some(x) -> str`
  * (Unwrap on wrong variant: type error in future; Phase-1 returns default/UB-guarded stubs.)

### Plan & Deliverables

* **AST**

  * `EnumDecl{Name string, Variants []EnumVariant}` with `EnumVariant{Name string, Type string /* "none" == void */}`.
* **Parser**

  * `enum` block with lines `Variant ":" Type`, permitting `none` for zero payload.
* **Checker**

  * `Info.Enums` table (like `Info.Structs`).
  * Validate constructors’ arity/kinds; record variant payload kinds.
  * Type for an enum value is the enum name (distinct from structs).
* **Codegen (C)**

  * Layout per enum:

    ```c
    typedef struct { int tag; union { int Ok; const char* Err; } as; } Result;
    ```

    (Tag values: 0,1,2… in declaration order.)
  * Constructors: inline functions (or macros) `Result Result_Ok(int)`, `Result Result_Err(const char*)`.
  * Predicates: `int Result_is_Ok(Result*)`, `int Result_is_Err(Result*)`.
  * Unwraps: `int Result_unwrap_Ok(Result*)`, etc. (no panics yet; may return default if tag mismatch).
* **Examples/Tests**

  * `examples/m8_enum_result.desi`:

    ```desi
    enum Result:
      Ok: int
      Err: str

    def main() -> int:
      let r: Result = Result.Ok(42)
      if Result.is_Ok(r):
        io.println("ok:", Result.unwrap_Ok(r))
      else:
        io.println("err:", Result.unwrap_Err(r))
      return 0
    ```
  * `examples/m8_enum_maybe.desi`:

    ```desi
    enum MaybeStr:
      Some: str
      None: none

    def main() -> int:
      let m: MaybeStr = MaybeStr.Some("hi")
      if MaybeStr.is_Some(m):
        io.println("some:", MaybeStr.unwrap_Some(m))
      else:
        io.println("none")
      return 0
    ```

### Future (not in Phase-1)

* `match` with exhaustiveness checks.
* Generics: `enum Result[T,E]: Ok: T; Err: E`.
* Zero-cost `unwrap` with compile-time tag knowledge in `match`.
* Better diagnostics on invalid unwraps.

---

## How to update

* Keep the table concise; put details under the milestone section.
* When you land work, add the example(s)/test(s) and flip the status.
* Cross-link deeper design docs (RFCS/spec) rather than duplicating content.
