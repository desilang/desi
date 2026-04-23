# Handle-Based QuerySet Architecture

**Status**: 🚧 In Progress (struct + lifecycle complete, compiler lowering pending)  
**Since**: v0.11  
**Related**: [Query Engine Improvements](query_engine_phase4.md), [ORM Models](orm_models.md), [Connection Pooling](connection_pooling.md)

---

## Overview

Phase 5 replaces the **global static state** in the ORM query engine with a **heap-allocated, handle-based `QuerySet` struct**. This eliminates the single largest source of concurrency bugs and memory safety issues in the C runtime.

### What Changed

| Before (Phase 4) | After (Phase 5) |
|---|---|
| ~30 `static` global variables | Single `QuerySet` struct per query chain |
| Fixed-size arrays (`char[2048]`) | Fixed-size embedded arrays with clear capacity limits |
| Fixed-size parameter arrays (`char*[64]`) | Dynamic `char**` arrays that grow via `realloc` |
| Global `__qs_reset()` clears everything | `__qs_new()` allocates, `__qs_free()` deallocates |
| Not reentrant, not thread-safe | Reentrant; thread-local shim for backward compat |

### Why

1. **Concurrency**: Desi supports `spawn`-based concurrency. Global ORM state meant two concurrent queries would corrupt each other.
2. **Memory safety**: Fixed `char*[64]` parameter arrays silently dropped parameters beyond index 64. Dynamic arrays grow as needed.
3. **Composability**: Handle-based design enables future features like subquery composition, query cloning, and deferred execution.

---

## Architecture

### QuerySet Struct (`crud.c`)

```c
typedef struct {
    // Core query components (fixed char arrays)
    char   table[256];
    char   where_clause[4096];
    char   order[1024];
    char   columns[2048];
    char   having[2048];
    char   group_by[512];

    // Scalar state
    int    limit, offset, distinct, row_count;

    // Dynamic parameter array (grows via realloc)
    char** params;
    int    param_count;
    int    param_cap;          // current capacity

    // INSERT field accumulator (dynamic)
    char** insert_keys;
    int    insert_count, insert_cap, insert_param_start;

    // UPDATE field accumulator (dynamic)
    char** update_keys;
    int    update_count, update_cap, update_param_start;

    // select_related (dynamic)
    char** related;
    int    related_count, related_cap;

    // Annotations (fixed array, max 8)
    Annotation annotations[QS_ANNOTATIONS_MAX];
    int    annotation_count;

    // Soft-delete
    int    soft_delete, include_deleted;
    char   soft_delete_col[128];

    // UPSERT + Window
    char   upsert_col[128];
    char   window_expr[1024];

    // Cursor, CTEs, Prefetch (fixed arrays)
    // ...
} QuerySet;
```

### Lifecycle

```
__qs_new("table_name")  →  QuerySet* (heap-allocated, zeroed)
       ↓
  chain methods use the handle (via thread-local shim for now)
       ↓
__qs_free(qs)           →  frees dynamic arrays + the struct itself
```

- `__qs_new()` uses `calloc` to zero-init all fields, then sets the table name and default soft-delete column via `strncpy`.
- `__qs_free()` iterates all dynamic arrays (`params`, `insert_keys`, `update_keys`, `related`), frees each element, frees the array, then frees the struct.

### Backward Compatibility Shim

To avoid rewriting every function body during the transition, a **thread-local shim** bridges old and new code:

```c
static __thread QuerySet* g_qs_current = NULL;

int32_t __qs_reset(const char* table) {
    if (g_qs_current) __qs_free(g_qs_current);
    g_qs_current = __qs_new(table);
    return g_qs_current ? 0 : -1;
}

// Macros redirect old global names → handle fields
#define qs_table        (g_qs_current->table)
#define qs_where        (g_qs_current->where_clause)
#define qs_order        (g_qs_current->order)
#define qs_params       ((const char**)g_qs_current->params)
#define qs_param_count  (g_qs_current->param_count)
// ... etc for all fields
```

This means existing function bodies like `__qs_filter()` compile unchanged — the macros transparently route reads and writes through the handle.

### Dynamic Arrays

Parameters, insert keys, update keys, and related tables all use a grow-on-demand pattern:

```c
static int qs_add_param(QuerySet* qs, const char* val) {
    if (qs->param_count >= qs->param_cap) {
        qs->param_cap *= 2;
        qs->params = realloc(qs->params, qs->param_cap * sizeof(char*));
    }
    qs->params[qs->param_count] = strdup(val);
    return ++qs->param_count;  // 1-based index
}
```

Initial capacities are tuned for typical queries:

| Array | Initial Capacity | Growth |
|---|---|---|
| `params` | 16 | 2× |
| `insert_keys` | 16 | 2× |
| `update_keys` | 16 | 2× |
| `related` | 8 | 2× |

### DynBuf Utility (`dynbuf.h`)

A standalone dynamic string buffer library is provided at `compiler/runtime/db/dynbuf.h` for future use. It offers:

- `dynbuf_init(buf, initial_cap)` — allocate with initial capacity
- `dynbuf_append(buf, str)` — append string, auto-grow
- `dynbuf_appendf(buf, fmt, ...)` — printf-style append
- `dynbuf_set(buf, str)` — overwrite contents
- `dynbuf_clear(buf)` — reset length to 0 without freeing
- `dynbuf_free(buf)` — release memory

> **Note**: The current QuerySet struct uses fixed `char[]` arrays for query components (where, order, etc.) rather than DynBuf. This preserves `sizeof()` compatibility with existing `snprintf` call sites. DynBuf is available for future use where truly unbounded string growth is needed.

---

## Files Changed

| File | Change |
|---|---|
| `compiler/runtime/db/dynbuf.h` | **[NEW]** Dynamic buffer utility |
| `compiler/runtime/db/crud.c` | **[MODIFIED]** QuerySet struct, lifecycle, shim macros, dynamic arrays |

---

## Pending Work (Phase 5 Continuation)

### Compiler Lowering (`lower_call.go`)

The compiler currently emits `__qs_reset("table")` at the start of every query chain. This needs updating to:

1. Emit `QuerySet* qs_N = __qs_new("table")` (returning a handle)
2. Thread `qs_N` as the first argument to every chained method call
3. Emit `__qs_free(qs_N)` after the terminal call (`fetch_all`, `update_exec`, etc.)

### Dispatch Layer (`dispatch.c`)

The dispatch functions (`__qs_dispatch_fetch`, etc.) need a `QuerySet*` parameter to route queries to the correct backend with the correct connection and state.

### API Bindings (`db.desi`)

Extern declarations need updating to include the handle as a `cptr` parameter. The public API wrappers will hide this from users.

---

## Design Decisions

1. **Fixed char[] over DynBuf for query components**: We chose embedded `char[]` arrays in the struct rather than heap-allocated DynBuf strings. The tradeoff is a fixed upper bound (e.g., 4096 bytes for WHERE) but guaranteed `sizeof()` correctness for all existing `snprintf(buf, sizeof(buf), ...)` calls. The sizes are generous — 4KB WHERE clauses and 2KB column lists cover virtually all real queries.

2. **Thread-local shim for migration path**: Rather than rewriting all ~80 function bodies at once, the `#define` shim lets the struct refactor land independently from the compiler lowering work. Each function will be migrated to accept `QuerySet*` directly in a future pass.

3. **calloc for zero-init**: `calloc` zeros all memory, so all `char[]` fields start as empty strings (first byte `'\0'`), all counts start at 0, and all pointers start as NULL. This eliminates an entire class of uninitialized-state bugs.

4. **strdup for parameter copies**: Each parameter value is copied via `strdup` so the caller's memory can be freed or reused. The QuerySet owns all parameter memory and frees it in `__qs_free()`.

---

## Testing Notes

- All 432 example tests pass (0–436)
- PostgreSQL ORM Phase 4 test (464) passes with handle-based QuerySet
- MySQL test (465) requires a running MySQL server (environment-dependent)
- All 11 C files in `compiler/runtime/db/` compile cleanly with clang
- Go build passes (`go build ./...`)
