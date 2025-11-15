# Desi Syntax (revised bootstrap)

This doc tracks the **implemented** subset at each milestone and calls out “what actually works today.” See the guides under `docs/guides/` for deeper details per milestone.

- **M2** extends M1 with:
  - **Decorators** on declarations (currently functions) and **docstring attachment** (first `"""..."""` in a block).
  - **Inline forms** for `if`/`while`/`for` (`if cond: stmt`, etc.).
  - **Using/Defer** statements.
  - **Augmented assignment** (`+=`, `-=`, `*=`, `/=` `%=` `**=` `^=`) and multi-target `:=`.
  - **Bitwise OR** `|` (now parsed, not just scanned).
- **M3A** adds **classes (parse-only)** with decorators, bases, fields, methods, nested classes, and docstrings.
- **M4** adds a **type-checking pass** on top of the existing grammar. See **[M4 — Types & Overloads](./guides/m4-types.md)** for the semantic rules.
- **M5** adds:
  - (Phase-1/2) **Imports** (`import` / `from … import …`), a loader with **multi-root** search, and a resolver that builds per-module **Exports** (functions only; **only** `pub def` with full types).
  - (Phase-3) **Module-qualified calls:** `import math; math.add(…)` resolves using the same exported signatures as `from math import add`. Non-exported member → **`DME0003`** (`"<mod> has no exported '<name>'"`).
  - (Phase-4) **Ergonomics v1:**
    **(4a)** `a + b` yields `str` if either side is `str` (coerces `int`/`float`/`bool`),
    **(4d)** Slice steps `s[i:j:k]` and short forms (`s[i:j]`, `s[:j]`, `s[:]`, `s[::k]`, `s[2::]`, …). In M5, **string slices type to `str`**; other containers will be typed later.
- **M6** adds a **function-local borrow checker** with `ref` / `inout` parameter modes and DBR diagnostics.
- **M7** adds **HIR lowering + Tier-0 LLVM IR** emission for `desic emit-ir`, including drops/arenas.
- **M8** wires in **async/futures**: async functions, `await`, join/with_timeout/gather/select lowering.
- **M9** adds **FFI v1** (C ABI) with `usize/isize`, `cptr[T]`, `unsafe:` blocks, and `@extern("C", …)` metadata.
- **M10** adds **Prelude v1 + Collections surface** (see **Prelude & collections (M10)** below).
- **M11** ships **Formatter v1** (token-based, idempotent) plus comment/docstring polish.
- **M12** ships **diagnostics polish & tooling**:
  - TTY renderer (`internal/diag/render`) with multi-label support and color policy.
  - JSON diagnostics mode (`--error-format=json`) emitting a **single JSON array** to stderr.
  - Global `--error-format` / `--color` flags for `desic` and `desifmt`.
- **M13** adds:
  - **Project manifest** `desi.mod` with `FindRoot`/`Load` in `internal/project`, and manifest-aware `desic init|run|build|test`.
  - **Named arguments** in function calls. Defaults and overloads are enforced at check time (see below).

---

## Type-checking (M4 overview)

M4 introduces a semantic/type pass that runs after parsing:

- **Concrete types**: `int`, `float`, `bool`, `str`, `none`; container shapes recognized in syntax (`list[T]`, `dict[K,V]`, `set[T]`, `tuple[...]`, `future[T]`) though checks vary by milestone.
- **Inference & checks**:
  - literals map to their scalar types; `let x = …` infers from RHS when no annotation is present.
  - binary arithmetic on `int|float` (exact operand types); comparisons produce `bool`; logical `and/or` require `bool`.
  - **pipeline `|>`** is checked as “insert LHS as the first argument”: `a |> f(b,c)` ≡ `f(a,b,c)`. Pipeline diagnostics mention “pipeline …” in messages for clarity.
  - **exact-match overloading** by **arity + parameter types**; ambiguous/missing picks report targeted diagnostics.
  - **parameter defaults**:
    - Functions may declare defaults in the signature: `def f(a: int, b: int = 1, c = 2) -> int: ...`.
    - Call sites may omit **trailing** parameters; missing ones are filled from defaults.
    - Defaults participate in overload matching; conflicting default layouts across overloads are rejected (see `DTE0102`).
  - **named arguments** (M13) are resolved per overload using the exported parameter names; bad names / duplicates get `DCA*` diagnostics.
  - **comprehensions** propagate element/key/value types into `list/set/dict`.
  - **lambdas**: typed parameters are supported (tests prefer explicit param types).
  - **multi-return** (via grouped assignment and returns): enforces **width** and element-wise compatibility.
  - **match (parse-only surface)**: all arm results must have the **same type** in this phase.

For full details and diagnostic codes, see **[M4 — Types & Overloads](./guides/m4-types.md)**.

---

## Calls & named arguments (M13)

M13 extends call syntax to support **named arguments** while keeping old positional calls 100% intact.

### Call forms

- **Positional-only** (pre-M13 behavior; still valid):
  - `f(1, 2, 3)`
- **Named + positional**:
  - `f(1, 2, z=3, w=4)`
- **Defaults** (M14 surface, checked in M4/M14 type phase):
  - `def f(a: int, b: int = 1, c = 2) -> int: ...`
  - `f(10)` uses defaults for `b` and `c`.
  - `f(10, c=5)` uses default for `b`, explicit value for `c`.

There is **no** `*args` / `**kwargs` / varargs in the language yet (those are planned for a later milestone).

### Rules

- Arguments are conceptually split into two zones: **leading positionals** and **trailing named**.
- Once the first **named** argument appears, **all following arguments must also be named**:
  - ✅ `f(1, 2, z=3, w=4)`
  - ❌ `f(1, z=2, 3)` → `DCA0003` (`positional argument after named arguments`)
- Named arguments are matched **per overload** using parameter names exported by:
  - the local function declaration (`def f(x: int, y: int) -> …`), or
  - resolver metadata for imported functions (e.g., `from math import add`).
- Defaults are applied **after** the named/positional mapping:
  - Omitted trailing parameters with defaults are filled.
  - You can skip a defaulted parameter by using a **named** argument on a later parameter:
    - `def f(a, b=1, c=2) -> int: ...`
    - `f(10, c=5)` is allowed; `b` uses its default.
- If, after mapping + defaults, **multiple overloads still fit**, the call is reported as **ambiguous** (`DTE0102`).

### Diagnostics (M13/M14)

- `DCA0001` — **unknown named argument**:
  - `f(x=1, z=2)` when `z` is not a parameter of any candidate.
- `DCA0002` — **duplicate named argument**:
  - `f(x=1, x=2)` or `f(1, x=2)` when the first positional already filled `x`.
- `DCA0003` — **positional argument after named arguments**:
  - `f(x=1, 2)`.
- `DTE0102` — **ambiguous overload** (including when defaults make multiple overloads equally good).

For calls that use **only positional arguments**, overload resolution behaves exactly as before M13; defaults only allow you to pass fewer arguments when the declaration has trailing defaults.

---

## Source form

- UTF-8, `\n` newlines. Shebang `#!` ignored if present top-of-file.

## Layout and indentation

Desi uses layout with `NL`, `Indent`, `Dedent`.
**Policy:** **tabs-only** at the start of a non-blank, non-comment line (spaces after the first token are fine).
*Blank lines and comment-only lines do not affect indentation.*

---

## Keywords

`import from as pub def async class struct enum type let mut return if elif else while for in using defer match select await true false none and or not`

> `in` is used both as the **membership operator** (see below) and as the clause keyword in `for … in …` and comprehensions.

---

## Prelude & collections (M10)

**Always-on prelude (no import needed):** `print`, `len`, `str`, `bool`, `range`, `map`, `filter`.

- **Shadowing guard:** defining a name that shadows a builtin yields **DPL0001** (`prelude.shadow_builtin`).
- **`len(x)` (v1):** `len("abc") -> int`. Unsupported types produce **DCO0001** with help/suggestions.
- **Membership `in` (v1):** `"a" in "abc" -> bool`. Other combos are **DCO0002** (unsupported membership).
  - Parser treats `in` as a binary operator at the **comparison** level **except** when parsing the *target* of `for … in …` or a comprehension clause (where `in` remains a keyword).
- **`range(start, stop, step=1)` (v1):** available for counted loops/comprehensions (lowered to a loop skeleton; no runtime protocol).
- **`map(xs,f)` / `filter(xs,p)` (v1):** surface sugar that **desugars to list comprehensions** before type-check **and** before `emit-ir`:
  - `map(xs,f)` → `[f(__x) for __x in xs]`
  - `filter(xs,p)` → `[__x for __x in xs if p(__x)]`
- **Collection stubs (compile-only):**
  - `list_push(inout list[T], T) -> none`
  - `set_add(inout set[T], T) -> none`
  - `dict_set(inout dict[K,V], K, V) -> none`
  These are used by lowering (Tier-0 textual IR) and have no runtime yet.

---

## Builtin types

`bool int i8 i16 i32 i64 i128 isize u8 u16 u32 u64 u128 usize f32 f64 str string future none list dict set tuple`

*(Some of these are “ready in syntax” but only partially used in the current backend; see the roadmap for when specific sizes become semantically meaningful.)*

## Literals

- Integers: dec/hex/bin/oct with `_` separators.
- Floats: decimal `1.2`, `2.`, `.5`, decimal exponents; **hex floats** `0x1.fp3`.
- Strings: `"..."`, `f"..."`, long `"""..."""` (triple-quoted).
  - Triple-quoted strings are recognized specially as **docstrings** when they are the **first statement in a block**; the parser emits a `StmtDocString` and, for **functions and classes**, attaches it to the decl.
  - **f-strings (stage 1):**
    - Lexically recognized via the `f` prefix (`f"..."`).
    - **Today** they are treated as plain `str` at type-time; `{…}` inside the string does **not** perform interpolation yet.
    - Proper interpolation semantics (Rust-style `Display`/`Debug` traits, typed holes) are scheduled under **M14**.

---

## Operators & punctuators (implemented today)

Greedy tokenization (longest wins).

- Grouping: `(` `)` `[` `]` `{` `}`
- Delimiters: `,` `:` `.` `@`
- Assignment: `=` `:=` `+=` `-=` `*=` `/=` `%=` `**=` `^=`
- Arithmetic: `+` `-` `*` `/` `%` `**`
- Bitwise/pipeline: `^` `|` `|>`
- Compare: `==` `!=` `<` `<=` `>` `>=` `in`
- Arrows: `->` `=>`
- Bang: `!`

### Expression precedence (high → low)

```

primary/index/call/field
unary
**                         # right-assoc

* / %

- -

<< >>
& ^ |
< <= > >= in
== !=
|>
and
or

```

> Membership `in` binds at the **comparison** tier. In clause contexts (`for X in Y`, or comprehension `... for X in Y ...`), `in` is a **clause keyword**, not an operator.

---

## Indexing & Slicing

* **Indexing:** `x[i]`
* **Slicing with steps (M5-4d):** `x[i:j:k]` supports any part omitted:

  * `x[i:j]`, `x[:j]`, `x[:]`, `x[::k]`, `x[2::]`, `x[1:5:2]`, etc.
* **Typing in M5:** if `x` is `str`, then `x[...]` is `str`. Other container slice typing will be added in a later milestone.

---

## M2 statements

* `let [mut] name [: Type] = Expr`
* `return [Expr]`
* `if Expr: SimpleStmt` or block form (`:` + NL + indented block). `elif`/`else` supported in both forms.
* `while Expr: SimpleStmt` or block form.
* `for Target in Expr: SimpleStmt` or block form.
  *Note:* the **Target** is parsed with `in` reserved for the clause (membership operator is disabled in that position).
* `using Expr: Block`
* `defer SimpleStmt` (executes at scope-exit)
* `match Expr: Block` with guarded arms (parse surface in M4; more checks later)

## Declarations

* `def name(params…) [-> Type]: Block` (decorators + docstrings supported; implicit `self` for class methods).

  * Parameters may have defaults: `x: int = 1`, `y = 2`.
  * Defaults should form a **suffix** of the parameter list; the checker applies defaults to omitted trailing args and participates in overload consistency/diagnostics.
* `class Name [ (Base, …) ]: Block` (parse-only checks in M3A; visibility policy in policy doc)
* `struct Name: Fields…`
* `enum Name: Variants…`
* `type Name = T`

---

## Imports (M5)

* `import dotted.name [as alias]` binds the **leaf** or `alias` as a **module binding** in the local scope.
* `from dotted.name import f [as g], …` binds selected items (**functions only**, if exported) directly in the local scope.
* **Packages:** a directory is a package iff it contains `__mod.desi` (leaf files may stand alone as modules).
* **Module-qualified calls (Phase-3):** `import math; math.add(2,3)` resolves via resolver **Exports**. Non-exported member → **`DME0003`**.
* **Re-exports (M14):** imports in a package’s `__mod.desi` are surfaced in the resolver’s **ModuleExports** so that:

  * `from math import add` works whether `add` is defined directly in `math/__mod.desi` or re-exported from `math/add.desi`.
* **Lints:** `DMW0004` unused module import; `DMW0005` unused from-item. Using a qualifier (`math.add`) counts as usage.
* Module-qualified calls share the same **named-argument** rules as plain calls (M13).

---

## CLI notes

* `desic check` exit codes: `0` ok, `1` had diagnostics, `2` arg/I/O error.
* `-I` supports **multi-root import search** (e.g., `-I "examples:compiler/lib"`).
* **Diagnostics output (M12):**

  * Human mode (TTY) uses a Rust-style renderer with caret/underline and color policy (`--color=auto|always|never`).
  * JSON mode (`--error-format=json`) emits **one JSON array** to **stderr** per run.
* **Manifest-aware CLIs (M13):**

  * `desic init|run|build|test` honor a `desi.mod` project manifest discovered via `FindRoot`.
  * Precedence for diagnostic settings: **CLI > env > manifest > defaults**.
* Diagnostic output is capped to a small number in the CLI with a suppression summary (keeps the console readable for very error-y files).

