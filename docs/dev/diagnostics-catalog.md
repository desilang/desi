# Diagnostics Catalog & Rendering Guide

> Single source of truth for compiler diagnostics.
> **Catalog:** `compiler/internal/diag/codes.json`
> **Renderer:** `compiler/internal/diag/render_tty.go`
> **Catalog API:** `compiler/internal/diag/catalog.go`

This document explains how Desi’s diagnostics are defined, rendered, and tested so **every tool** (`desic check`, `emit-ir`, `build`, etc.) shows consistent, helpful messages.

---

## 1) Code anatomy

Every diagnostic has a stable, unique **CodeID**, e.g.:

- **Lexer:** `DLE0001`
- **Parser:** `DPE0005`, `DPE0110`, `DPE1003`
- **Type checker:** `DTE0004`
- **Borrow checker:** `DBR0004`
- **Module/Resolver:** `DMO00xx` (grouped as `module` in the catalog)
- **Warnings:** `DW0001`, `DW0007`

**Severity:**
The TTY renderer treats codes beginning with `DW` as **warnings**; everything else is an **error**.

**Stability:**
CodeIDs are **never recycled**. If we change semantics, add a new CodeID.

---

## 2) The catalog (`codes.json`)

The catalog is a hierarchical JSON map:

```json
{
  "lexer": {
    "<slug>": Entry,
    ...
  },
  "parser": {
    "<slug>": Entry,
    ...
  },
  "type": {
    "<slug>": Entry,
    ...
  },
  "module": {
    "<slug>": Entry,
    ...
  },
  "warn": {
    "<slug>": Entry,
    ...
  },
  "borrow": {
    "<slug>": Entry,
    ...
  }
}
```

Each **Entry** looks like:

```json
{
  "id": "DPE0110",
  "title": "missing 'let' before variable declaration",
  "help": "Write 'let name = …' to declare a variable; plain '=' is not a statement operator.",
  "suggestions": [
    {
      "where": {
        "kind": "at",
        "role": "primary"
      },
      "label": "insert 'let '",
      "message": "Add 'let ' before the name.",
      "replacement": "let ",
      "applicability": "machine-applicable"
    }
  ],
  // Optional: a rendering hint some lex/scan errors use
  "primary_end": {
    "kind": "at",
    "role": "primary"
  }
}
```

### Fields

* `id` (**required**): The globally unique CodeID.
* `title`: Short, human-readable problem statement (sentence-style, lowercase unless a proper noun).
* `help`: One actionable guidance sentence (present tense, imperative voice).
* `suggestions[]` (optional): Quick-fix candidates

  * `where.kind`: `"at" | "eol" | "bol" | "range"`
  * `where.role`: `"primary"` if anchored at diagnostic’s primary span
  * `label`: Short verb phrase for the fix (shown first)
  * `message`: Optional extra explanation
  * `replacement`: Text inserted/applied by tools
  * `applicability`: `"machine-applicable" | "maybe-applicable" | "unspecified"`
* `primary_end` (optional): extra hint for caret placement used by some lexer diagnostics.

---

## 3) Rendering pipeline

1. **Compiler emits** a `diag.Diagnostic` with:

* `CodeID` (e.g., `"DPE0110"`),
* `Domain` (e.g., `"parser"`),
* **Primary** span (and any secondary labels/notes).
* Leave `Title` and `Help` **empty** unless you have a genuine per-site override.

2. **Renderer** (`render_tty.go`) calls `FillFromCatalog(&d)`:

* If `d.Title`/`d.Help` are empty, they are filled from the catalog entry for `d.CodeID`.
* If the diagnostic has no suggestions, the renderer **shows catalog suggestions**.

3. Output example:

```
error[DPE0110] missing 'let' before variable declaration
  --> examples/15_m8_async_basic.desi:11:5
  = help: Write 'let name = …' to declare a variable; plain '=' is not a statement operator.
  = suggestion (at primary): insert 'let ' — Add 'let ' before the name. [machine-applicable]
```

* **`(at primary)`** means the fix applies at the diagnostic’s **primary span**.
* **`[machine-applicable]`** means a tool can safely auto-apply the fix.

---

## 4) Catalog access API (Go)

`compiler/internal/diag/catalog.go` exposes:

* `LoadCatalog(io.Reader) (Catalog, error)` — decode a catalog (used by some tests).
* `Lookup(id string) (Entry, bool)` — get entry by **CodeID** (e.g., `"DTE0004"`).
* `Known(id string) bool` — true if the **CodeID** exists.
* `FillFromCatalog(d *Diagnostic)` — populate `Title`/`Help` if empty.

Implementation detail:

* The catalog is **embedded** with `//go:embed codes.json`.
* We build a reverse **ID index** for O(1) lookup.
* JSON decoding is strict (`DisallowUnknownFields()`), so unexpected fields in `codes.json` will fail tests.

---

## 5) Adding a new diagnostic (recipe)

1. **Pick group** (`lexer` / `parser` / `type` / `borrow` / `module` / `warn`) and next available numeric.
2. **Edit** `compiler/internal/diag/codes.json` and add an `Entry`:

* Tight `title`, one-sentence `help`, opt-in `suggestions`.

3. **Emit** a `diag.Diagnostic` in Go:

* Set `CodeID` and the **primary span**.
* Leave `Title` and `Help` empty so the renderer uses the catalog.

4. **Tests**:

* Add/adjust a golden or unit test to assert rendering includes the **code**, **title**, and **help** (and suggestions if applicable).
* Run `go test ./...`.

**Example** (Parser: missing `let` before var decl — `DPE0110`):

* `codes.json` → new entry under `"parser"` (as above).
* Parser calls:

  ```go
  diag.Diagnostic{
      CodeID: "DPE0110",
      Domain: "parser",
      Primary: diag.Label{Span: sp, Primary: true},
  }
  ```
* Renderer fills title/help/suggestion from the catalog.

---

## 6) Conventions & style

* **Titles:** sentence-style, concise; avoid punctuation unless needed.
* **Help:** single sentence, present-tense imperative, tells the user **what to do**.
* **Suggestions:** `label` is a short action (“insert 'let '”), `message` explains why.
* **IDs:** don’t reuse; keep stable across releases.
* **Groups:** match the logical owner:

  * `lexer` scanning/tokenization
  * `parser` syntax/structure
  * `type` type system, overloads, signatures
  * `borrow` ownership/borrowing
  * `module` imports/resolution
  * `warn` non-fatal issues (`DW…`)

---

## 7) Guardrails & tests

* **Unknown CodeIDs forbidden:**
  `compiler/internal/diag/diag_codes_references_test.go` scans `compiler/internal/**.go` for `CodeID:"..."` and fails if the ID isn’t in `codes.json` (`Known(id) == false`).
* **Catalog presence tests:** unit tests assert specific codes (like `DPE0110`) exist.
* **Strict JSON:** adding unexpected keys to `codes.json` will fail decoding.

**Run:**

```bash
go build ./... && go test ./...
```

---

## 8) FAQ

**Q: When should I embed a message directly in Go?**
A: Almost never. Prefer catalog-only content. Only embed a message if the text truly must vary by site (then still keep a catalog entry for common fields).

**Q: Can I add multi-step suggestions?**
A: Yes; use `suggestions[]` with multiple entries. Keep each atomic and machine-applicability honest.

**Q: How do warnings show up?**
A: Codes starting with `DW` render as `warning[DWxxxx] …`. Everything else renders as `error[...]`.

---

*Last updated: keep this doc in sync when adding groups/fields or changing the renderer.*

