# Desi Syntax (revised bootstrap)

This doc tracks the **implemented** subset at each milestone and calls out “what actually works today.” See the guides under `docs/guides/` for deeper details per milestone.

- **M2** extends M1 with:
  - **Decorators** on declarations (currently functions) and **docstring attachment** (first `"""..."""` in a block).
  - **Inline forms** for `if`/`while`/`for` (`if cond: stmt`, etc.).
  - **Using/Defer** statements.
  - **Augmented assignment** (`+=`, `-=`, `*=`, `/=`, `%=`, `**=`, `^=`) and multi-target `:=`.
  - **Bitwise OR** `|` (now parsed, not just scanned).
- **M3A** adds **classes (parse-only)** with decorators, bases, fields, methods, nested classes, and docstrings.
- **M4** adds a **type-checking pass** on top of the existing grammar. See **[M4 — Types & Overloads](./guides/m4-types.md)** for the semantic rules.
- **M5** adds:
  - (Phase-1/2) **Imports** (`import` / `from … import …`), a loader with **multi-root** search, and a resolver that builds per-module **Exports** (functions only; **only** `pub def` with full types).
  - (Phase-3) **Module-qualified calls:** `import math; math.add(…)` resolves using the same exported signatures as `from math import add`. Non-exported member → **`DME0003`** (`"<mod> has no exported '<name>'"`).
  - (Phase-4) **Ergonomics v1:**
    **(4a)** `a + b` yields `str` if either side is `str` (coerces `int`/`float`/`bool`),
    **(4d)** Slice steps `s[i:j:k]` and short forms (`s[i:j]`, `s[:j]`, `s[:]`, `s[::k]`, `s[2::]`, …). In M5, **string slices type to `str`**; other containers will be typed later.

---

## Type-checking (M4 overview)

M4 introduces a semantic/type pass that runs after parsing:

- **Concrete types**: `int`, `float`, `bool`, `str`, `none`; container shapes recognized in syntax (`list[T]`, `dict[K,V]`, `set[T]`, `tuple[...]`, `future[T]`) though checks vary by milestone.
- **Inference & checks**:
  - literals map to their scalar types; `let x = …` infers from RHS when no annotation is present.
  - binary arithmetic on `int|float` (exact operand types); comparisons produce `bool`; logical `and/or` require `bool`.
  - **pipeline `|>`** is checked as “insert LHS as the first argument”: `a |> f(b,c)` ≡ `f(a,b,c)`. Pipeline diagnostics mention “pipeline …” in messages for clarity.
  - **exact-match overloading** by **arity + parameter types**; ambiguous/missing picks report targeted diagnostics.
  - **comprehensions** propagate element/key/value types into `list/set/dict`.
  - **lambdas**: typed parameters are supported (tests prefer explicit param types).
  - **multi-return** (via grouped assignment and returns): enforces **width** and element-wise compatibility.
  - **match (parse-only surface)**: all arm results must have the **same type** in this phase.

For full details and diagnostic codes, see **[M4 — Types & Overloads](./guides/m4-types.md)**.

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

## Builtin types

`bool int i8 i16 i32 i64 i128 isize u8 u16 u32 u64 u128 usize f32 f64 str string future none list dict set tuple`

## Literals

- Integers: dec/hex/bin/oct with `_` separators.
- Floats: decimal `1.2`, `2.`, `.5`, decimal exponents; **hex floats** `0x1.fp3`.
- Strings: `"..."`, `f"..."`, long `"""..."""` (triple-quoted).
  - Triple-quoted strings are recognized specially as **docstrings** when they are the **first statement in a block**; the parser emits a `StmtDocString` and, for **functions and classes**, attaches it to the decl.
  - **f-strings (stage 1):** treated as plain `str` at type-time; hole parsing/desugaring is deferred.

## Operators & punctuators (implemented today)

Greedy tokenization (longest wins).

- Grouping: `(` `)` `[` `]` `{` `}`
- Delimiters: `,` `:` `.` `@`
- Assignment: `=` `:=` `+=` `-=` `*=` `/=` `%=` `**=` `^=`
- Arithmetic: `+` `-` `*` `/` `%` `**`
- Bitwise/pipeline: `^` `|` `|>`
- Compare: `==` `!=` `<` `<=` `>` `>=`
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
< <= > >=
== !=
|>
and
or

```

---

## Indexing & Slicing

- **Indexing:** `x[i]`
- **Slicing with steps (M5-4d):** `x[i:j:k]` supports any part omitted:
  - `x[i:j]`, `x[:j]`, `x[:]`, `x[::k]`, `x[2::]`, `x[1:5:2]`, etc.
- **Typing in M5:** if `x` is `str`, then `x[...]` is `str`. Other container slice typing will be added in a later milestone.

---

## M2 statements

- `let [mut] name [: Type] = Expr`
- `return [Expr]`
- `if Expr: SimpleStmt` or block form (`:` + NL + indented block). `elif`/`else` supported in both forms.
- `while Expr: SimpleStmt` or block form.
- `for Target in Expr: SimpleStmt` or block form.
- `using Expr: Block`
- `defer SimpleStmt` (executes at scope-exit)
- `match Expr: Block` with guarded arms (parse surface in M4; more checks later)

## Declarations

- `def name(params…) [-> Type]: Block` (decorators + docstrings supported; implicit `self` for class methods)
- `class Name [ (Base, …) ]: Block` (parse-only checks in M3A; visibility policy in policy doc)
- `struct Name: Fields…`
- `enum Name: Variants…`
- `type Name = T`

---

## Imports (M5)

- `import dotted.name [as alias]` binds the **leaf** or `alias` as a **module binding** in the local scope.
- `from dotted.name import f [as g], …` binds selected items (**functions only**, if exported) directly in the local scope.
- **Packages:** a directory is a package iff it contains `__mod.desi` (leaf files may stand alone as modules).
- **Module-qualified calls (Phase-3):** `import math; math.add(2,3)` resolves via resolver **Exports**. Non-exported member → **`DME0003`**.
- **Lints:** `DMW0004` unused module import; `DMW0005` unused from-item. Using a qualifier (`math.add`) counts as usage.

---

## CLI notes

- `desic check` exit codes: `0` ok, `1` had diagnostics, `2` arg/I/O error.
- `-I` supports **multi-root import search** (e.g., `-I "examples:compiler/lib"`).
- Diagnostic output is capped to a small number in the CLI with a suppression summary (keeps the console readable for very error-y files).

