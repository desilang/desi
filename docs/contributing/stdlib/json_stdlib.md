# JSON Stdlib Implementation

This document describes the internal implementation of the `json` standard library module.

## Architecture

The JSON module uses a **hybrid C+Desi architecture**:

```
Desi Code → Type Checker → Lowering → HIR → LLVM Emit → C Runtime
                ↓              ↓           ↓              ↓
         expr_call.go    lower_call.go   emit_call.go   json.c
```

## Components

### 1. C Runtime (`compiler/runtime/json.c`)

The C runtime handles all JSON parsing and stringification:

**Core Types:**
```c
typedef enum { JSON_NULL, JSON_BOOL, JSON_NUMBER, JSON_STRING, JSON_ARRAY, JSON_OBJECT } JsonType;

typedef struct JsonNode {
    JsonType type;
    union {
        int bool_val;          // JSON_BOOL
        double num_val;        // JSON_NUMBER
        char* str_val;         // JSON_STRING
        JsonArray array;       // JSON_ARRAY
        JsonObject object;     // JSON_OBJECT
    };
} JsonNode;
```

**Key Functions:**
| C Function | Desi API | Returns |
|------------|----------|---------|
| `__json_parse(ptr)` | `json.parse()` | `ptr` (JsonNode*) |
| `__json_stringify(ptr)` | `json.stringify()` | `ptr` (string) |
| `__json_is_null(ptr)` | `json.is_null()` | `i32` → `i1` |
| `__json_is_int(ptr)` | `json.is_int()` | `i32` → `i1` |
| `__json_get_int(ptr)` | `json.get_int()` | `i64` |
| `__json_get_number(ptr)` | `json.get_number()` | `double` |
| `__json_get_string(ptr)` | `json.get_string()` | `ptr` |
| `__json_array_get(ptr, i32)` | `json.array_get()` | `ptr` |
| `__json_object_get(ptr, ptr)` | `json.object_get()` | `ptr` |
| `__json_object_key(ptr, i32)` | `json.object_key()` | `ptr` |
| `__json_new_object()` | `json.new_object()` | `ptr` |
| `__json_new_array()` | `json.new_array()` | `ptr` |
| `__json_new_string(ptr)` | `json.new_string()` | `ptr` |
| `__json_new_number(double)` | `json.new_number()` | `ptr` |
| `__json_new_bool(i32)` | `json.new_bool()` | `ptr` |
| `__json_new_null()` | `json.new_null()` | `ptr` |
| `__json_object_set(ptr, ptr, ptr)` | `json.set()` | `void` |
| `__json_array_push(ptr, ptr)` | `json.push()` | `void` |
| `__json_object_remove(ptr, ptr)` | `json.remove()` | `void` |
| `__json_object_keys(ptr)` | `json.keys()` | `ptr` |
| `__json_free(ptr)` | internal | `void` |

### 2. Type Checker (`compiler/internal/check/expr_call.go`)

All `json.*` calls are handled specially around line 69-118:

```go
if id, ok := fe.X.(*ast.Ident); ok && id.Name == "json" {
    method := fe.Name.Name
    // ... validate import
    switch method {
    case "is_null", "is_bool", "is_number", "is_string", "is_array", "is_object", "is_int":
        c.info.Types[call] = types.Bool
        return types.Bool
    case "get_int":
        c.info.Types[call] = types.Int
        return types.Int
    // ... etc
    }
}
```

### 3. Lowering (`compiler/internal/lower/lower_call.go`)

Lowers `json.*` calls to HIR Call instructions. Each builder maps to the corresponding C function:

```go
// Builder example
if method == "new_object" && len(x.Args) == 0 {
    dst := ls.b.FreshTemp("json_obj")
    ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_new_object", Args: nil, Type: "ptr"})
    return dst
}

// Void mutation example
if method == "set" && len(x.Args) >= 3 {
    objVal := ls.lowerExpr(x.Args[0])
    keyVal := ls.lowerExpr(x.Args[1])
    valVal := ls.lowerExpr(x.Args[2])
    ls.b.Emit(&hir.Call{Fn: "__json_object_set", Args: []hir.Value{objVal, keyVal, valVal}})
    return hir.ConstNull{}
}
```

### 4. Sig Overrides (`compiler/internal/backend/llvm/sig_overrides.go`)

All `__json_*` functions are registered in `sig_overrides.go` with correct LLVM return types. This prevents the LLVM backend from defaulting to `i32`:

```go
SetFuncSig("__json_new_object", "ptr", nil)
SetFuncSig("__json_object_set", "void", nil)
// ... etc (19 total)
```

**Why this is needed:** Without sig overrides, the LLVM IR generator declares unknown extern functions as `declare i32 @fn(...)`, causing type mismatches when the caller expects `ptr` (string/node pointer).

### 5. LLVM Emit (`compiler/internal/backend/llvm/emit_call.go`)

Emits correct LLVM IR for each C function. Example:

```go
if c.Fn == "__json_get_int" && len(c.Args) == 1 {
    m.ensureDecl("declare i64 @__json_get_int(ptr)")
    dst := c.Dst.Name
    _, nodeVal := m.operand(c.Args[0])
    wprintf(&m.funcs, "  %s = call i64 @__json_get_int(ptr %s)\n", dst, nodeVal)
    m.tempTypes[strings.TrimPrefix(dst, "%")] = "i64"
    return
}
```

**Important:** Type tracking in `m.tempTypes` is critical for:
- F-string formatting (uses correct printf format)
- Branch instructions (uses correct LLVM type)

### 6. Type Tracking Pattern

For functions returning non-i32 types, store in `tempTypes`:

```go
// After emitting call:
if m.tempTypes == nil {
    m.tempTypes = make(map[string]string)
}
m.tempTypes[strings.TrimPrefix(dst, "%")] = "i64"  // or "double", "ptr", "i1"
```

This prevents `inferType()` from defaulting to `i32`.

## Builder Design

Builders create new `JsonNode*` on the heap via `calloc`/`malloc`:

- **Ownership:** caller owns the node. Nodes set via `object_set`/`array_push` become children of the parent.
- **Update semantics:** `object_set` on existing key frees the old value before replacing.
- **Remove:** `object_remove` frees key + value and shifts remaining entries.
- **Keys:** `object_keys` returns a NEW array of NEW string nodes (caller owns both).

## Adding New JSON Functions

1. **C Runtime:** Add function in `json.c`
2. **Sig Override:** Register in `sig_overrides.go` with correct return type
3. **Type Checker:** Add case in `expr_call.go` json switch
4. **Lowering:** Add handler in `lower_call.go` json block
5. **LLVM Emit:** Add emit handler in `emit_call.go`
6. **Desi API:** Add wrapper in `json.desi`
7. **Test:** Add test case in example file
8. **Document:** Update learner (`docs/stdlib/json.md`) and contributor docs

## Number Handling Design

The hybrid Python+Rust approach:

1. **`is_int(n)`** - Checks if `floor(n) == n` (no decimal part)
2. **`get_int(n)`** - Returns `(int64_t)num_val` 
3. **`get_float(n)`** - Returns `num_val` as double

This gives:
- Clean integer output: `42` instead of `42.000000`
- Type safety: explicit accessor choice
- Flexibility: use either depending on context

## Files

| File | Purpose |
|------|---------|
| `compiler/runtime/json.c` | C runtime (parser, stringify, builders) |
| `compiler/lib/json.desi` | Desi API wrappers |
| `compiler/internal/check/expr_call.go` | Type checking |
| `compiler/internal/lower/lower_call.go` | HIR lowering |
| `compiler/internal/backend/llvm/emit_call.go` | LLVM IR emit |
| `compiler/internal/backend/llvm/sig_overrides.go` | LLVM return type overrides |
| `docs/stdlib/json.md` | Learner documentation |
| `docs/contributing/stdlib/json_stdlib.md` | Contributor documentation |

