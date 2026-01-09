# Collections Runtime Internals

This document explains the implementation of Desi's collection types for contributors.

## Overview

All collections are heap-allocated and use type-dispatched operations:

| Collection | Runtime File | Key Type Tags |
|------------|--------------|---------------|
| `list[T]` | `list.c` | Element type via function pointer |
| `set[T]` | `set.c` | Integer hash-based |
| `dict[K, V]` | `dict.c` | TYPE_TAG_INT/STR/BOOL/FLOAT/CUSTOM |

## Memory Ownership Model

### Owned vs Borrowed

Collections **own** their elements. When a collection is freed, elements are freed too.

**Critical**: `dict.get` returns a **borrowed reference**. The compiler tracks this:

```go
// lower_stmt.go: Mark dict.get result as borrowed
if call, isCallExpr := s.Value.(*ast.CallExpr); isCallExpr {
    if field, ok := call.Callee.(*ast.FieldExpr); ok && field.Name.Name == "get" {
        if _, isDict := baseType.(*types.Dict); isDict {
            ls.cur().borrowed[s.Name.Name] = true  // Skip drop
        }
    }
}
```

### Why This Matters

Without borrowing semantics, this code would double-free:

```desi
let cache: dict[str, Item] = {}
cache.insert("k", Item(42))
let item = cache.get("k", Item(0))  # Returns borrowed ref
# End of scope:
# - item would be freed (WRONG - it's borrowed)
# - cache freed (frees its entries, including item)
# = Double free!
```

## Dict Custom Keys

### Type Tags

```c
#define TYPE_TAG_INT    0
#define TYPE_TAG_STR    1
#define TYPE_TAG_BOOL   2
#define TYPE_TAG_FLOAT  3
#define TYPE_TAG_CUSTOM 4  // Classes, structs, enums
```

### Pointer Identity vs Value Equality

For custom keys (TYPE_TAG_CUSTOM), behavior depends on function pointers:

| `key_hash_fn` | `key_eq_fn` | Behavior |
|---------------|-------------|----------|
| NULL | NULL | Pointer identity (default) |
| Provided | Provided | Value-based (user-defined) |

```c
// hash_key() - uses pointer address when hash_fn is NULL
if (d->key_hash_fn == NULL) {
    return (uint64_t)(uintptr_t)key_ptr;
}
return d->key_hash_fn(key_ptr);

// keys_equal() - pointer comparison when eq_fn is NULL
if (d->key_eq_fn == NULL) {
    return entry->key_ptr == key_ptr;
}
return d->key_eq_fn(entry->key_ptr, key_ptr);
```

### Enum Key Behavior

> [!IMPORTANT]
> Each access to `Color.Red` allocates a new object. Store in variable first.

## Dict Value Types

### Pointer Type Casting

`dict.get` returns values through a `void*`. For pointer types (str, class, list, set, dict), the compiler casts i64 → ptr:

```go
// dict_lower.go: Cast for pointer value types
switch valType.(type) {
case *types.Class, *types.List, *types.Set, *types.Dict, *types.Struct:
    needsPtrCast = true
}
if needsPtrCast {
    ls.b.Emit(&hir.Cast{Src: valDst, Dst: ptrDst, Type: "ptr"})
}
```

Without this, LLVM IR would use i64 where ptr is expected → type error.

## Set Implementation

Sets use integer hash tables. For custom types, they use pointer identity (same as dict with NULL hash/eq).

Set iteration order is **not guaranteed** (elements are prepended to buckets for O(1) insert).

## Test Coverage

| Test File | Coverage |
|-----------|----------|
| 274-276 | list/set with custom types |
| 277-280 | Nested collections |
| 281-283 | Dict literals, dict.get |
| 284-285 | Double-free fix, comprehensive combinations |

## Related Files

- `compiler/runtime/dict.c`, `list.c`, `set.c` - C runtime
- `compiler/internal/lower/dict_lower.go` - Dict lowering
- `compiler/internal/lower/lower_stmt.go` - Borrowed tracking (line 230+)
