# Dict Internals

This document explains the implementation of Desi's dict type, focusing on generic key support.

## Overview

Desi's `dict[K, V]` supports primitive key types: `int`, `str`, `bool`, `float`. The implementation uses type-dispatched hashing and comparison to handle different key types efficiently.

## Phase 1: Primitive Keys (Current)

### Supported Key Types

| Type | Hash Function | Comparison |
|------|---------------|------------|
| `int` | `(uint64_t)key_value` | Direct `==` |
| `str` | FNV-1a hash | `strcmp()` |
| `bool` | `(uint64_t)key_value` | Direct `==` |
| `float` | Bit representation | `memcmp()` (bit-exact) |

### Runtime Structure

```c
// dict.h
typedef struct dict_entry {
    int64_t key_int;    // Inline storage for int/bool
    char* key_str;      // String key (owned, strdup'd)
    double key_float;   // Float key
    void* value;        // Generic value pointer
    struct dict_entry* next;
} dict_entry_t;

typedef struct dict {
    dict_entry_t** buckets;
    size_t bucket_count;
    size_t entry_count;
    size_t value_size;
    int key_type_tag;      // 0=int, 1=str, 2=bool, 3=float
    int value_type_tag;
    ElemToStrFunc value_to_str_fn;
} dict_t;
```

### Type Tags

```c
#define TYPE_TAG_INT   0
#define TYPE_TAG_STR   1
#define TYPE_TAG_BOOL  2
#define TYPE_TAG_FLOAT 3
```

### Compiler Integration

The lowering phase (`lower_collections.go`) determines key type at compile time and passes it to `dict_new()`:

```go
// Extract key type tag from dict type
keyTypeTag = getTypeTag(dictType.Key)

// dict_new(key_type_tag, value_size, value_type_tag, to_str_fn)
ls.b.Emit(&hir.Call{Fn: "dict_new", Args: []hir.Value{
    keyTypeTag, valueSize, valTypeTag, toStrFunc
}})
```

### Key Handling in `in` Operator

The `in` operator lowering (`lower_expr.go`) dispatches based on key type:

```go
if isStrKey {
    keyStr = lhs  // Use string key
} else if isFloatKey {
    keyFloat = lhs  // Use float key
} else {
    // int or bool - cast to i64
    keyInt = cast(lhs, "i64")
}
// dict_has_key(dict, key_int, key_str, key_float)
```

## Float Key Behavior

> [!IMPORTANT]
> Float keys use **bit-exact comparison**, not epsilon comparison.
> - `0.0` and `-0.0` are different keys
> - `NaN != NaN` (NaN keys won't match themselves)

This is consistent with Python's dict behavior and ensures hash table invariants.

## Phase 2: Custom Types (Future)

To support user-defined types as keys, classes will need to implement `__hash__` and `__eq__` dunders:

```desi
class Point:
    x: int
    y: int
    
    pub def __hash__(self) -> int:
        return self.x * 31 + self.y
    
    pub def __eq__(self, other: Point) -> bool:
        return self.x == other.x and self.y == other.y
```

## Memory Management

- **String keys**: Allocated with `strdup()`, freed in `dict_free()`
- **Integer/Bool keys**: Stored inline (no allocation)
- **Float keys**: Stored inline (no allocation)
- **Values**: All values allocated as 8-byte slots

## Testing

Test files in `examples/`:
- `268_dict_in_operator.desi` - All primitive key types
- `269_nested_dict.desi` - Dict values containing dicts
- `150_dict_iteration.desi` - `for k, v in dict.items():`
