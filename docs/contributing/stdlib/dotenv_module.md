# Dotenv Module Implementation

Internal documentation for the `dotenv` standard library module.

## Architecture

```
compiler/lib/dotenv.desi     → Desi API (load, get)
compiler/runtime/dotenv.c    → C runtime (file parsing, setenv)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__dotenv_load` | `dotenv.load` | `(const char*) → int32` |
| `__dotenv_load_or_fail` | `dotenv.load_or_fail` | `(const char*) → int32` |
| `__dotenv_parse` | `dotenv.parse` | `(const char*) → char*` |
| `__dotenv_get` | `dotenv.get` | `(const char*) → char*` |

## Implementation Notes

- `load()` silently ignores missing files (returns 0). `load_or_fail()` returns -1.
- The parser sets environment variables via POSIX `setenv()`.
- `get()` is a thin wrapper around `getenv()`.
- Comments (lines starting with `#`) and empty lines are skipped.
- Supports `KEY=VALUE`, `KEY="VALUE"`, and `KEY='VALUE'` formats.
- No handles needed — environment is global process state.

## Test Coverage

- `examples/504_dotenv_module.desi`
