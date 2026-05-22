# JSON Stdlib Implementation

This document describes the internal implementation of the `json` standard library module.

## Architecture

The JSON module uses a **decoupled C+Desi architecture** — the same pattern as HTTP and math modules. There are NO special-case interceptors in the lowerer.

```
User Code                     json.desi                         C Runtime
json.parse(text)  →  pub def parse(text) →  @extern("C") __json_parse(text)  →  json.c
json.is_null(n)   →  pub def is_null(n)  →  __json_type(n) == JSON_NULL       →  json.c
json.set(o,k,v)   →  pub def set(o,k,v) →  @extern("C") __json_object_set    →  json.c
```

### Key Design Decision: Decoupled from Lowerer

Previously, `lower_call.go` had a ~192-line interceptor block that hard-coded JSON function dispatch. This was removed in May 2026 and replaced with direct `@extern("C")` calls from the Desi module — the same technique used for the HTTP module decoupling.

**Benefits:**
- Adding new JSON functions requires NO compiler changes — just C + Desi
- User `@extern("C")` wrappers use the exact same code path as stdlib
- Fixes silent `int` → `bool` type mismatches in C returns

## Components

### 1. Desi Module (`compiler/lib/json.desi`)

The module has two sections:

**C Runtime Bindings** (~30 `@extern("C")` declarations):
```desi
@extern("C")
pub def __json_parse(text: str) -> Any

@extern("C")
def __json_new_object() -> Any

@extern("C")
def __json_object_set(obj: Any, key: str, val: Any)
```

**Public API** (~40 `pub def` wrappers):
```desi
pub def parse(text: str) -> Any:
    unsafe:
        return __json_parse(text)

pub def is_null(node: Any) -> bool:
    unsafe:
        return __json_type(node) == JSON_NULL
```

Note: `is_null/is_bool/is_number/is_string/is_array/is_object` are computed in Desi as `__json_type(node) == CONSTANT` rather than calling dedicated C functions. This is cleaner and avoids extra C function calls.

### 2. C Runtime (`compiler/runtime/json.c`)

**Core Types:**
```c
typedef enum { JSON_NULL, JSON_BOOL, JSON_NUMBER, JSON_STRING, JSON_ARRAY, JSON_OBJECT } JsonType;

typedef struct JsonNode {
    JsonType type;
    union {
        bool bool_val;         // JSON_BOOL (was int, fixed May 2026)
        double num_val;        // JSON_NUMBER
        char* str_val;         // JSON_STRING
        JsonArray array;       // JSON_ARRAY
        JsonObject object;     // JSON_OBJECT
    };
} JsonNode;
```

**Key Functions:**
| C Function | Desi API | C Returns | LLVM Type |
|------------|----------|-----------|-----------|
| `__json_parse(ptr)` | `json.parse()` | `JsonNode*` | `ptr` |
| `__json_stringify(ptr)` | `json.stringify()` | `char*` | `ptr` |
| `__json_type(ptr)` | `json.get_type()` | `int` | `i32` |
| `__json_is_int(ptr)` | `json.is_int()` | `bool` | `i1` |
| `__json_get_bool(ptr)` | `json.get_bool()` | `bool` | `i1` |
| `__json_get_int(ptr)` | `json.get_int()` | `int` | `i32` |
| `__json_get_number(ptr)` | `json.get_number()` | `double` | `double` |
| `__json_get_string(ptr)` | `json.get_string()` | `char*` | `ptr` |
| `__json_equals(ptr, ptr)` | `json.equals()` | `bool` | `i1` |
| `__json_has_key(ptr, ptr)` | `json.has_key()` | `bool` | `i1` |
| `__json_new_object()` | `json.new_object()` | `JsonNode*` | `ptr` |
| `__json_object_set(ptr, ptr, ptr)` | `json.set()` | `void` | `void` |

### 3. LLVM Backend (`emit_call.go`)

The backend still has interceptor blocks for JSON C functions. These handle:
- **Type declarations**: `declare i1 @__json_is_int(ptr)` etc.
- **Type tracking**: Store result types in `m.tempTypes` so later code knows the correct LLVM type
- **Argument type conversion**: e.g., `trunc i64 to i32` for array indices

These backend interceptors are NOT the same as the removed lowerer interceptors. The lowerer interceptors bypassed the Desi module entirely. The backend interceptors just ensure correct LLVM IR emission for the `@extern("C")` functions.

### 4. Type Tracking Pattern

For functions returning non-i32 types, the backend stores the result type in `tempTypes`:

```go
// After emitting call:
m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"  // or "i32", "double", "i1"
```

This prevents `inferType()` from defaulting to `i32`, which would cause type mismatches when the result is passed to another function.

## Adding New JSON Functions

After decoupling, adding new JSON functions requires NO compiler changes:

1. **C Runtime:** Add function in `json.c`
2. **Desi Module:** Add `@extern("C")` declaration + `pub def` wrapper in `json.desi`
3. **Test:** Add test case in example file
4. **Document:** Update learner docs (`book/docs/stdlib/json.md`) and this file

If the function returns a non-standard type (not `i32`), you may also need:
5. **Backend:** Add an interceptor in `emit_call.go` with correct `ensureDecl` and `tempTypes`

## Builder Design

Builders create new `JsonNode*` on the heap via `calloc`/`malloc`:

- **Ownership:** caller owns the node. Nodes set via `object_set`/`array_push` become children of the parent.
- **Update semantics:** `object_set` on existing key frees the old value before replacing.
- **Remove:** `object_remove` frees key + value and shifts remaining entries.
- **Keys:** `object_keys` returns a NEW array of NEW string nodes (caller owns both).

## Number Handling Design

The hybrid Python+Rust approach:

1. **`is_int(n)`** - Checks if `floor(n) == n` (no decimal part)
2. **`get_int(n)`** - Returns `(int32_t)num_val`
3. **`get_float(n)`** - Returns `num_val` as double

This gives:
- Clean integer output: `42` instead of `42.000000`
- Type safety: explicit accessor choice
- Flexibility: use either depending on context

## Files

| File | Purpose |
|------|---------|
| `compiler/runtime/json.c` | C runtime (parser, stringify, builders) |
| `compiler/lib/json.desi` | Desi API wrappers + `@extern("C")` declarations |
| `compiler/internal/backend/llvm/emit_call.go` | LLVM IR emit (type declarations + tracking) |
| `book/docs/stdlib/json.md` | Learner documentation |
| `docs/contributing/stdlib/json_stdlib.md` | This file |
