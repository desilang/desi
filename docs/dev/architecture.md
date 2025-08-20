# Architecture (High-Level)

```

source
└─ lexer (indent/newline aware)
└─ tokens (with INDENT/DEDENT)
└─ parser (Pratt expressions + statement parser)
└─ AST (typed nodes with spans)
└─ resolve (scopes, symbols)
└─ typecheck (inference for lets, generics mono)
└─ IR (structured control flow)
└─ lower (ARC insertion, escape analysis)
└─ codegen/c (emit portable C)
└─ system C compiler + runtime → executable

```

## Packages (Stage-0)
- `lexer` — indentation-aware scanner; emits NEWLINE/INDENT/DEDENT
- `parser` — builds AST; reports diagnostics with spans
- `ast` — node types
- `resolve` — scopes/symbols (Stage-0 may inline some into parser)
- `types` — type representations, monomorphization helpers (added later)
- `check` — type & flow checks
- `ir` — Desi-IR nodes (structured)
- `lower` — ARC & escapes; later SSA
- `codegen/c` — C emitter
- `runtime` — C runtime: ARC, strings, vec, channels

## Data flow
- The compiler carries a `Session` that owns interning tables, file maps, and a diagnostics sink.
- Stages pass typed data; avoid untyped `map[string]any`.

## Error handling
- Never panic on user input. Return `[]diag.Diagnostic` and continue where possible to gather more errors.
