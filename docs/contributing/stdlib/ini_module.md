# INI Module Implementation

Internal documentation for the `ini` standard library module.

## Architecture

```
compiler/lib/ini.desi     → Desi API (parse, query)
compiler/runtime/ini.c    → C runtime (INI parser)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__ini_parse` | `ini.parse` | `(const char*) → IniDoc*` |
| `__ini_parse_file` | `ini.parse_file` | `(const char*) → IniDoc*` |
| `__ini_get` | `ini.get` | `(IniDoc*, char*, char*) → char*` |
| `__ini_get_default` | `ini.get_default` | `(IniDoc*, char*, char*, char*) → char*` |
| `__ini_has_section` | `ini.has_section` | `(IniDoc*, char*) → int32` (→ bool) |
| `__ini_has_key` | `ini.has_key` | `(IniDoc*, char*, char*) → int32` (→ bool) |
| `__ini_sections` | `ini.sections` | `(IniDoc*) → char*` (comma-separated) |
| `__ini_keys` | `ini.keys` | `(IniDoc*, char*) → char*` (comma-separated) |
| `__ini_free` | `ini.free` | `(IniDoc*) → int32` |

## Implementation Notes

- The parser is a single-pass line-by-line scanner.
- Sections are stored as a linked list of key-value pair arrays.
- `sections()` and `keys()` return comma-separated strings (the Desi API does not yet support returning `list<str>` from C).
- Handles are `cptr` — opaque pointers to `IniDoc` structs.

## Test Coverage

- `examples/507_ini_module.desi`
