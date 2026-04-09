# SQLite3 Module Implementation

This document covers the internal implementation details of the `sqlite3` module for contributors.

## Architecture

```
compiler/lib/sqlite3/__mod.desi   ← Desi public API (16 functions)
         ↓ (extern "C" calls)
compiler/runtime/sqlite3.c       ← Thin C wrapper (result caching)
         ↓ (calls)
compiler/runtime/sqlite3_amalg.c  ← Full SQLite3 engine (embedded)
compiler/runtime/sqlite3_amalg.h  ← SQLite3 headers
```

## Key Design: Embedded Amalgamation

Unlike most SQLite3 integrations that link against a system library, Desi **embeds the entire SQLite3 source** (260K lines) directly into `libdesi.a`. This means:

- **Zero external dependencies** — works on any platform without installing SQLite3
- **Consistent behavior** — same SQLite3 version across all deployments
- **Single binary** — no shared library resolution at runtime

## C Runtime Implementation

The wrapper (`sqlite3.c`) maintains global state:

```c
static sqlite3 *_db = NULL;           // Active database connection
static char **_result_data = NULL;     // Cached query results
static char **_result_columns = NULL;  // Column names
static int _result_rows = 0;          // Row count
static int _current_row = -1;         // Cursor position
```

### Query Result Caching

`__sqlite3_query()` calls `sqlite3_get_table()` which fetches ALL rows into memory at once. The wrapper then provides cursor-based iteration:

1. `query("SELECT ...")` → fetches all rows, stores in `_result_data`, returns count
2. `fetch_next()` → increments `_current_row`, returns false at end
3. `get("name")` → scans `_result_columns` for matching name, returns `_result_data[offset]`

### Limits

- Max 1024 rows per query result (compile-time constant, can be increased)
- Max 64 columns per query
- Single database connection at a time

## Build Integration

In the `Makefile`:
- `sqlite3_amalg.c` compiles to `build/sqlite3_amalg.o` (~10s on first build)
- `sqlite3.c` compiles to `build/sqlite3.o`
- Both are added to `libdesi.a`

**Important**: `sqlite3.c` must `#include "sqlite3_amalg.h"` **before** declaring any `sqlite3*` typed variables.

## API Naming

`connect()`/`disconnect()` are used instead of `open()`/`close()` because `open` is a Desi compiler builtin (in `desiBuiltins` map) that forces a `ptr` return type — causing LLVM IR type mismatch when the function actually returns `i32`.
