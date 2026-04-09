# sqlite3 Module Implementation

The `sqlite3` module provides an embedded SQL database using the official SQLite3 amalgamation — **zero external dependencies**.

## Architecture

```
import sqlite3
    ↓
compiler/lib/sqlite3/__mod.desi  →  @extern("C") bindings
    ↓
compiler/runtime/sqlite3.c  →  Thin C wrapper (__sqlite3_*)
    ↓
compiler/runtime/sqlite3_amalg.c  →  Full SQLite3 engine (260K lines)
compiler/runtime/sqlite3_amalg.h  →  SQLite3 headers
```

## Files

| File | Purpose |
|------|---------|
| `compiler/lib/sqlite3/__mod.desi` | Desi bindings (16 pub def functions) |
| `compiler/runtime/sqlite3.c` | C wrapper with result caching and row iteration |
| `compiler/runtime/sqlite3_amalg.c` | Official SQLite3 amalgamation (public domain) |
| `compiler/runtime/sqlite3_amalg.h` | SQLite3 header file |

## C Runtime Functions

| C Function | Desi Binding | Return | Description |
|-----------|-------------|--------|-------------|
| `__sqlite3_open(path)` | `connect(path)` | `int` | Open/create database |
| `__sqlite3_close()` | `disconnect()` | `int` | Close connection |
| `__sqlite3_execute(sql)` | `execute(sql)` | `int` | Run non-query SQL |
| `__sqlite3_query(sql)` | `query(sql)` | `int` | Run SELECT, returns row count |
| `__sqlite3_fetch_next()` | `fetch_next()` | `bool` | Advance cursor |
| `__sqlite3_get_field(col)` | `get_field(col)` | `str` | Get by column index |
| `__sqlite3_get_field_by_name(name)` | `get(name)` | `str` | Get by column name |
| `__sqlite3_row_count()` | `row_count()` | `int` | Result row count |
| `__sqlite3_col_count()` | `col_count()` | `int` | Result column count |
| `__sqlite3_col_name(idx)` | `col_name(idx)` | `str` | Column name |
| `__sqlite3_last_insert_id()` | `last_insert_id()` | `int` | Last rowid |
| `__sqlite3_changes()` | `changes()` | `int` | Rows affected |
| `__sqlite3_last_error()` | `last_error()` | `str` | Error message |
| `__sqlite3_begin()` | `begin()` | `int` | Begin transaction |
| `__sqlite3_commit()` | `commit()` | `int` | Commit transaction |
| `__sqlite3_rollback()` | `rollback()` | `int` | Rollback transaction |

## Design Decisions

- **Embedded amalgamation**: The full SQLite3 source (sqlite3_amalg.c/h) is compiled directly into libdesi.a. No runtime dependency on libsqlite3.
- **API naming**: `connect()`/`disconnect()` instead of `open()`/`close()` — `open` is a compiler builtin that forces ptr return type.
- **Result caching**: `__sqlite3_query()` fetches ALL rows into memory (`_result_data` array) and provides cursor-based iteration via `fetch_next()`. Max 1024 rows × 64 columns.
- **Single connection**: Global `_db` pointer — one database connection at a time. Future: connection handle parameter.
- **All values as strings**: SQLite natively works with text; all `get()` calls return `str`. Type conversion (int, float) is done in Desi code.

## Build Notes

- `sqlite3_amalg.c` compiles to `build/sqlite3_amalg.o` — takes ~10s on first build
- `sqlite3.c` **must** include `sqlite3_amalg.h` before declaring any variables that use `sqlite3*` type
- The Makefile compiles both as separate .o files, both go into libdesi.a

## Cross-Platform

- The SQLite amalgamation is highly portable — works on macOS, Linux, Windows, ARM, x86
- No platform-specific code in the wrapper (sqlite3.c)
- On Windows, SQLite uses its own file locking implementation

## Future Improvements

- Parameterized queries (prevent SQL injection)
- Multiple concurrent connections (connection handle)
- Prepared statements for repeated queries
- BLOB support (currently text-only)
