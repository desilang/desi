# Desi Guides

Practical, example-driven docs. Start here:

- **M0 — Lexing & Literals:** [m0-basics.md](./m0-basics.md)
  Numbers (incl. hex floats), strings & escapes, operators, layout, diagnostics, CLI usage.

- **M1 — Parser & AST:** [m1-parser.md](./m1-parser.md)
  Functions/blocks, statements, expressions & precedence, postfix chains, async `def`.

- **M4 — Types & Overloads:** [m4-types.md](./m4-types.md)
  Type pass, exact-match overloading (arity + types), pipelines `|>`, lambdas, multi-return.

- **M5 — Imports & Resolver:** [m5-imports.md](./m5-imports.md)
  Packages via `__mod.desi`, multi-root `-I`, Exports (functions only; `pub def` + full types),
  **Phase-3:** *module-qualified calls* (`import math; math.add(…)`) with `DME0003` on non-exports.

- **M5 — Ergonomics v1:** [m5-ergonomics.md](./m5-ergonomics.md)
  **4a:** `str` on `+` when either side is `str` (coerces int/float/bool).
  **4d:** Slice steps `s[i:j:k]` and short forms. In M5, **string slices type to `str`**.

- **Module Function Mangling:** [module-mangling.md](./module-mangling.md)
  How `__desi$` name mangling prevents C symbol collisions for module functions,
  how to add new stdlib modules, and how to register new builtins.

---

## CLI quick refs

```bash
# Check a file (multi-root import search)
go run ./compiler/cmd/desic check -I "examples:compiler/lib" -v examples/13_m5_imports_qualified.desi

# Token or AST demos
go run ./compiler/cmd/desic -demo-tokens
go run ./compiler/cmd/desic -ast examples/07_ast_smoke.desi
```

> The `check` subcommand caps printed diagnostics for readability and shows a suppression line if more exist.
