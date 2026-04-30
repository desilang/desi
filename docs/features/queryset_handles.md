# Handle-Based QuerySet Architecture

**Status**: ✅ Implemented  
**Since**: v0.11  
**Related**: [Query Engine Improvements](query_engine.md), [ORM Models](orm_models.md), [Connection Pooling](connection_pooling.md)

---

## Overview

The handle-based QuerySet architecture replaces the **global static state** in the ORM query engine with a **heap-allocated, handle-based `QuerySet` struct**. This eliminates the single largest source of concurrency bugs and memory safety issues in the C runtime.

### What Changed

| Before | After |
|---|---|
| ~30 `static` global variables | Single `QuerySet` struct per query chain |
| Fixed-size arrays (`char[2048]`) | Fixed-size embedded arrays with clear capacity limits |
| Fixed-size parameter arrays (`char*[64]`) | Dynamic `char**` arrays that grow via `realloc` |
| Global `__qs_reset()` clears everything | `__qs_new()` allocates, `__qs_free()` deallocates |
| Not reentrant, not thread-safe | Reentrant; thread-local shim for backward compat |
| Compiler emits `__qs_reset()` implicitly | Compiler emits explicit `new → bind → use → free` |

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
    char   order[512];
    char   columns[4096];
    char   having[2048];
    char   group_by[1024];

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

### Handle-Threading API

The compiler emits explicit handle lifecycle calls. Four C-level functions manage handles:

| Function | Signature | Purpose |
|---|---|---|
| `__qs_handle_new(table)` | `const char* → void*` | Allocate a new `QuerySet`, return opaque handle |
| `__qs_handle_bind(qs)` | `void* → void` | Install handle as active for subsequent `__qs_*` calls |
| `__qs_handle_free(qs)` | `void* → void` | Free handle; clear thread-local if it matches |
| `__qs_handle_get()` | `→ void*` | Return current active handle (introspection) |

**Lifecycle:**

```
__qs_handle_new("users")   →  void* (heap-allocated QuerySet)
         ↓
__qs_handle_bind(qs)       →  install as active (thread-local)
         ↓
  __qs_filter / __qs_order_by / ...  (read from active handle via shim macros)
         ↓
  __qs_fetch / __qs_delete / ...     (terminal: execute SQL)
         ↓
__qs_handle_free(qs)       →  free dynamic arrays + struct, clear thread-local
```

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

## Compiler Integration

### Lowering (`lower_call.go`)

The compiler's `resolveModelObjectsChain` function emits explicit handle management at the chain root:

```llvm
; Chain root — allocate and bind
%qs_handle = call ptr @__qs_handle_new(ptr @.str.users)
call void @__qs_handle_bind(ptr %qs_handle)

; Intermediate — filter (reads g_qs_current via shim)
call i32 @__qs_filter(ptr @.str.name, ptr @.str.Alice)

; Terminal — fetch and cleanup
%qs_result = call i32 @__qs_fetch()
call void @__qs_handle_free(ptr %qs_handle)
```

The handle value is threaded through all recursive chain resolution, so `emitQsGeneric` receives it and emits the `__qs_handle_free()` after the terminal call completes.

### ABI Bindings (`db.desi`)

Extern declarations expose the handle API to the Desi type system:

```desi
@extern("C")
pub def __qs_handle_new(table: str) -> cptr

@extern("C")
pub def __qs_handle_bind(qs: cptr) -> int

@extern("C")
pub def __qs_handle_free(qs: cptr) -> int

@extern("C")
pub def __qs_handle_get() -> cptr
```

### Dispatch Layer

The dispatch layer (`dispatch.c`) routes finished SQL strings to PG/MySQL backends and does **not** need `QuerySet*` awareness. The CRUD layer converts handle state → SQL before calling dispatch.

---

## Files Changed

| File | Change |
|---|---|
| `compiler/runtime/db/dynbuf.h` | **[NEW]** Dynamic buffer utility |
| `compiler/runtime/db/crud.c` | **[MODIFIED]** QuerySet struct, lifecycle, handle-threading API, shim macros, dynamic arrays |
| `compiler/internal/lower/lower_call.go` | **[MODIFIED]** Emit `__qs_handle_new/bind/free` in query chain lowering |
| `compiler/lib/db.desi` | **[MODIFIED]** Added handle-threading extern declarations |

---

## Design Decisions

1. **Fixed char[] over DynBuf for query components**: We chose embedded `char[]` arrays in the struct rather than heap-allocated DynBuf strings. The tradeoff is a fixed upper bound (e.g., 4096 bytes for WHERE) but guaranteed `sizeof()` correctness for all existing `snprintf(buf, sizeof(buf), ...)` calls. The sizes are generous — 4KB WHERE clauses and 4KB column lists cover virtually all real queries.

2. **Thread-local shim for migration path**: Rather than rewriting all ~80 function bodies at once, the `#define` shim lets the struct refactor land independently from the compiler lowering work. Each function will be migrated to accept `QuerySet*` directly in a future pass.

3. **calloc for zero-init**: `calloc` zeros all memory, so all `char[]` fields start as empty strings (first byte `'\0'`), all counts start at 0, and all pointers start as NULL. This eliminates an entire class of uninitialized-state bugs.

4. **strdup for parameter copies**: Each parameter value is copied via `strdup` so the caller's memory can be freed or reused. The QuerySet owns all parameter memory and frees it in `__qs_free()`.

5. **Compiler-driven lifecycle**: The compiler emits explicit `new → bind → free` calls rather than relying on implicit reset/cleanup. This makes handle ownership deterministic and visible in the IR, enabling future `using`-scope support.

---

## Testing Notes

- 456 of 461 example tests pass (5 pre-existing migration/MySQL failures unrelated to handle refactor)
- PostgreSQL ORM test (464) compiles and runs with handle-based QuerySet
- MySQL test (465) requires a running MySQL server (environment-dependent)
- All C files in `compiler/runtime/db/` compile cleanly with clang (0 warnings)
- Go build passes (`go build ./...`)
- `libdesi.a` rebuilt with handle-threading symbols (`__qs_handle_new/bind/free/get`)
