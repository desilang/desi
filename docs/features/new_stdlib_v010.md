# New Stdlib Modules (v0.1.0)

## csv Module
**Files:** `compiler/runtime/csv.c`, `compiler/lib/csv.desi`

RFC 4180 compliant CSV parsing/writing. C runtime handles the heavy lifting with proper quote escaping, the Desi wrapper provides a clean public API.

### Architecture
- `__csv_parse(data)` → Splits on newlines (respecting quoted fields), then splits each line on commas with `parse_csv_line()`
- `__csv_format(rows)` → Iterates rows, auto-quotes fields containing commas/quotes/newlines, escapes `"` as `""`
- File I/O: `__csv_read_file` reads entire file, passes to `__csv_parse`. `__csv_write_file` formats then writes.

### API
| Function | Signature |
|---|---|
| `csv.parse` | `(data: str) -> list[list[str]]` |
| `csv.parse_delim` | `(data: str, delim: str) -> list[list[str]]` |
| `csv.format` | `(rows: list[list[str]]) -> str` |
| `csv.read_file` | `(path: str) -> list[list[str]]` |
| `csv.write_file` | `(path: str, rows: list[list[str]]) -> int` |

---

## encoding Module
**Files:** `compiler/runtime/encoding.c`, `compiler/runtime/base64.c`, `compiler/lib/encoding.desi`

Unifies hex and base64 encoding under one import. Hex functions in `encoding.c`, base64 functions were pre-existing in `base64.c`.

### API
| Function | Signature |
|---|---|
| `encoding.hex_encode` | `(data: str) -> str` |
| `encoding.hex_decode` | `(hex: str) -> str` |
| `encoding.hex_encode_upper` | `(data: str) -> str` |
| `encoding.base64_encode` | `(data: str) -> str` |
| `encoding.base64_decode` | `(data: str) -> str` |
| `encoding.base64_url_encode` | `(data: str) -> str` |
| `encoding.base64_url_decode` | `(data: str) -> str` |

---

## template Module
**Files:** `compiler/runtime/template.c`, `compiler/lib/template.desi`

Simple `{{key}}` placeholder substitution engine. Uses parallel `keys`/`values` lists to avoid needing dict FFI at the C level.

### Architecture
- `__template_render(tmpl, keys, values)` — Scans for `{{...}}`, trims whitespace around key name, looks up in keys list, replaces with corresponding value. Unknown keys are preserved.
- `__template_render_pairs(tmpl, pairs)` — Splits a flat `[k1, v1, k2, v2, ...]` list into keys/values, then delegates to `__template_render`.
- `__template_escape_html(input)` — Replaces `&`, `<`, `>`, `"`, `'` with HTML entities.

### API
| Function | Signature |
|---|---|
| `template.render` | `(tmpl: str, keys: list[str], values: list[str]) -> str` |
| `template.render_pairs` | `(tmpl: str, pairs: list[str]) -> str` |
| `template.escape_html` | `(input: str) -> str` |

---

## Parameter Defaults Fix (M14)
**File:** `compiler/internal/lower/lower_call.go`

The parser and type checker already supported parameter defaults (`def foo(x: int, y: int = 10)`), but the lowerer was not injecting default values at call sites when fewer arguments were passed. Fixed by resolving the callee's `FuncDecl` via `ls.info.Funcs` at the fallthrough call path and lowering default expressions for omitted parameters.
