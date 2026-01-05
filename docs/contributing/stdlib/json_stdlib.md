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

Lowers `json.*` calls to HIR Call instructions around line 774-880:

```go
if method == "get_int" && len(x.Args) >= 1 {
    nodeVal := ls.lowerExpr(x.Args[0])
    dst := ls.b.FreshTemp("json_int")
    ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_get_int", Args: []hir.Value{nodeVal}, Type: "i64"})
    return dst
}
```

### 4. LLVM Emit (`compiler/internal/backend/llvm/emit_call.go`)

Emits correct LLVM IR for each C function around line 130-250:

```go
if c.Fn == "__json_get_int" && len(c.Args) == 1 {
    m.ensureDecl("declare i64 @__json_get_int(ptr)")
    dst := c.Dst.Name
    _, nodeVal := m.operand(c.Args[0])
    wprintf(&m.funcs, "  %s = call i64 @__json_get_int(ptr %s)\n", dst, nodeVal)
    // Track type for f-string formatting
    m.tempTypes[strings.TrimPrefix(dst, "%")] = "i64"
    return
}
```

**Important:** Type tracking in `m.tempTypes` is critical for:
- F-string formatting (uses correct printf format)
- Branch instructions (uses correct LLVM type)

### 5. Type Tracking Pattern

For functions returning non-i32 types, store in `tempTypes`:

```go
// After emitting call:
if m.tempTypes == nil {
    m.tempTypes = make(map[string]string)
}
m.tempTypes[strings.TrimPrefix(dst, "%")] = "i64"  // or "double", "ptr", "i1"
```

This prevents `inferType()` from defaulting to `i32`.

## Adding New JSON Functions

1. **C Runtime:** Add function in `json.c`
2. **Type Checker:** Add case in `expr_call.go` json switch
3. **Lowering:** Add handler in `lower_call.go` json block
4. **LLVM Emit:** Add emit handler in `emit_call.go`
5. **Test:** Add test case in example file
6. **Document:** Update learner and contributor docs

## Number Handling Design

The hybrid Python+Rust approach:

1. **`is_int(n)`** - Checks if `floor(n) == n` (no decimal part)
2. **`get_int(n)`** - Returns `(int64_t)num_val` 
3. **`get_float(n)`** - Returns `num_val` as double

This gives:
- Clean integer output: `42` instead of `42.000000`
- Type safety: explicit accessor choice
- Flexibility: use either depending on context

## Files Changed

| File | Purpose |
|------|---------|
| `compiler/runtime/json.c` | C runtime functions |
| `compiler/internal/check/expr_call.go` | Type checking |
| `compiler/internal/lower/lower_call.go` | HIR lowering |
| `compiler/internal/backend/llvm/emit_call.go` | LLVM IR emit |
| `book/docs/stdlib/json.md` | Learner documentation |
| `docs/dev/json_stdlib.md` | Contributor documentation |
