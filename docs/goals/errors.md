# Goal: Rust-like diagnostics in Desi

## Phase 0 — Baseline (✅ done)

* JSON **codes registry** (`codes.json`) with IDs, titles, default help, shaping (`primary_end`), and default suggestions.
* Go renderer that prints Rust-style messages with primary/secondary spans + suggestions.
* Bridge fallback that infers a key for legacy `LEXERR … msg="..."` lines.

## Phase 1 — Lock the structured diagnostic schema

Define a single schema the Desi toolchain will emit (lexer/parser/typechecker), and the Go bridge can already consume.

**Fields (minimal, stable):**

* `domain`: `"lexer" | "parser" | "type" | "other"`
* `key`: stable registry key, e.g. `"unterminated_string"`, `"use_after_move"`
* `level`: `"error" | "warning" | "note"`
* `code`: optional override (else derived from registry)
* `message`: optional override (else registry title)
* `primary`: `{ file, line, col, end_col? }`
* `secondaries`: array of spans `{ file, line, col, end_col?, label? }`
* `help`: optional, appended after registry help
* `notes`: array of strings
* `suggestions`: array `{ at:{file,line,col,end_col?}, label?, message?, replacement?, applicability? }`

**Wire format:** extend the current NDJSON to include structured `DIAG` rows, e.g.

```json
{"type":"DIAG",
 "domain":"lexer",
 "key":"unterminated_string",
 "level":"error",
 "primary":{"file":"examples/bad_lex.desi","line":1,"col":9},
 "notes":[],
 "suggestions":[{"at":{"file":"examples/bad_lex.desi","line":1,"col":22},
                 "label":"add this: '\"'",
                 "message":"insert closing quote",
                 "replacement":"\"",
                 "applicability":"machine-applicable"}]
}
```

Keep `LEXERR …` as a temporary compatibility event until the lexer emits `DIAG`.

## Phase 2 — Bridge & renderer updates (small Go changes)

* **Lexbridge**:

  * Parse `DIAG` rows; **prefer** their `(domain,key)` and spans.
  * If only legacy `LEXERR` is present, keep today’s substring fallback.
* **Renderer**:

  * Hydrate `(domain,key)` via `codes.json` (already implemented).
  * Merge in per-instance fields (message override, help, secondaries, suggestions).
  * Keep shaping (`primary_end`) and suggestion locators **data-driven**.

> Outcome: once the Desi side starts emitting `DIAG`, Go needs **no changes** to show rich errors.

## Phase 3 — Implement diagnostics in Desi (self-hosted)

Create a small Desi library:

* `compiler/desi/diag.desi`
  Types: `Span`, `Suggestion`, `Diagnostic`, `Level`, `Applicability`.
* `compiler/desi/diag_registry.desi`
  Loader for `codes.json` (or a generated `.desi` map at build time).
* `compiler/desi/diag_emitter.desi`
  Functions to build `Diagnostic` with `(domain, key)` and append notes/secondaries/suggestions.
* `compiler/desi/diag_ndjson.desi`
  Serialize `Diagnostic` as NDJSON `DIAG` rows to stdout/stderr for the bridge.

**In each compiler stage (Desi code):**

* **Lexer**: emit `DIAG` (`domain="lexer"`) with `key` + primary span; add default suggestion (or let registry do it).
* **Parser**: on “expected X, found Y”, build `DIAG` with secondaries pointing at the token(s) causing the issue.
* **Typechecker**: examples:

  * `use_after_move` (DTE0001): primary at later use, secondary at move site, note at declaration, suggestion `.clone()` with applicability.
  * `mismatch` (DTE0002): primary at expression, secondary at declaration of expected type, suggestion to `:type` annotate or cast.

## Phase 4 — UX refinements (optional but nice)

* **Color** output (ANSI), with `--no-color` flag and TTY detection.
* **Path elision**: shorten long paths (e.g., repo-relative) in headers.
* **Tab/Unicode** alignment\*\*: already handled (visual column logic); keep it in Desi too.
* **`desic --explain CODE`**: print longform explanation (pull from a doc map).
* **`--fix`**: apply machine-applicable suggestions (in-memory rewrite + write-back).
* **i18n**: keep `codes.json` translatable (e.g., `title_en`, `title_xx`; renderer picks locale).

## Phase 5 — Testing strategy

* **Golden tests** for renderer: input `Diagnostic` → exact text output (with/without color).
* **Integration fixtures**: tiny `.desi` files that trigger one well-known diagnostic; assert output contains code ID, correct spans & labels.
* **Registry validation**: CI check that all `id` are unique, all `key` names are canonical, and suggestions only use supported locators.

## Minimal Go changes required (summary)

* What we already did: JSON registry + Rust-style renderer.
* Still needed later:

  1. **NDJSON parser** accepts `DIAG` rows (non-breaking).
  2. **Bridge** forwards DIAGs to the renderer (non-breaking).
  3. Optionally keep substring fallback for older toolchains.

Everything else (semantics, messages, suggestions) lives in **Desi** + `codes.json`.
