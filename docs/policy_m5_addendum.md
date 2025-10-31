# Desi Language — Policy Addendum for M5

This addendum records the *enforced* behaviors and constraints introduced in M5 (Resolver & Imports Phases 1–4).

## 1) Export Surface (functions only, typed only)

- A module exports **only** functions declared as `pub def` **with fully annotated parameters and return**.
- Non-function symbols (classes, structs, enums, consts, types) are **not** exported cross-module in M5.
- Re-exports and wildcard imports (`from pkg import *`) are **out of scope** for M5.

## 2) Packages and Resolution

- A directory is a package **iff** it contains `__mod.desi`.
- Resolution honors multi-root search (`-I root1:root2:…`) and probes packages before leaf files.
- Binding rules:
  - `import a.b.c [as C]` binds `c` (or `C`) as a **module binding** in local scope.
  - `from a.b import f [as g]` binds `f` (or `g`) directly as a local callable.

## 3) Qualified Calls Policy

- `mod.fn(args…)` is permitted **only** when `mod` is a local **import binding**.
- Candidates for `fn` are taken from the resolver’s **Exports** for that module.
- If `fn` is missing/private/untyped, emit **`DME0003 module.bad_import`** at the field name span with message:
  `"<mod> has no exported '<name>'"`.

## 4) Lints

- **`DMW0004`** (unused import) is **not** emitted if the module appears as a qualifier (e.g., `math.add(...)`).
- **`DMW0005`** (unused from-item) remains unaffected.

## 5) Ergonomics v1 (Phase-4) Policy

- **Implicit `str` on `+`:** If either operand is `str`, the result type is `str`. The other operand may be `int|float|bool|str`. No other implicit conversions are introduced in M5.
- **Slice steps:** Forms `x[i:j:k]`, `x[i:j]`, `x[:j]`, `x[:]`, `x[::k]`, `x[2::]` parse.
  In M5, **only** `str` slices are typed (`str → str`); other container slice typing is deferred.

## 6) Diagnostics & CLI Output

- The compiler emits all diagnostics (including cascades).
- The `desic check` CLI **caps printed diagnostics to 15** and prints a suppression summary (e.g., “... N more errors suppressed”) to preserve terminal readability.

## 7) Known Limitation (to be tightened later)

- If a local variable shadows an import name (e.g., `math = 42`), qualified resolution currently still treats `math` as a module binding for `math.fn`. Avoid such shadowing in M5; a scope-aware fix is planned post-M5.
