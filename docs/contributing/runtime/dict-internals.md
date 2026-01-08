# Dict Internals

This document explains the implementation of Desi's dict type for contributors.

## Architecture Overview

Desi's `dict[K, V]` uses a hash table with chained buckets. It supports:
- **Primitive keys**: `int`, `str`, `bool`, `float` (stored inline)
- **Custom keys**: classes, structs, enums (pointer-based or value-based)

## Type Tags

```c
#define TYPE_TAG_INT    0
#define TYPE_TAG_STR    1
#define TYPE_TAG_BOOL   2
#define TYPE_TAG_FLOAT  3
#define TYPE_TAG_CUSTOM 4  // Classes, structs, enums
```

## Runtime Structure

```c
typedef struct dict_entry {
    int64_t key_int;     // Int/bool key (inline)
    char* key_str;       // String key (owned, strdup'd)
    double key_float;    // Float key (inline)
    void* key_ptr;       // Custom key (owned or reference)
    void* value;         // Generic value pointer
    struct dict_entry* next;
} dict_entry_t;

typedef struct dict {
    dict_entry_t** buckets;
    size_t bucket_count, entry_count, value_size;
    int key_type_tag, value_type_tag;
    size_t key_size;           // For copying custom keys
    KeyHashFunc key_hash_fn;   // NULL = pointer identity
    KeyEqFunc key_eq_fn;       // NULL = pointer equality
    ElemToStrFunc value_to_str_fn;
} dict_t;
```

## Custom Key Behavior

### Default: Pointer Identity

When `key_hash_fn` and `key_eq_fn` are NULL (no `__hash__`/`__eq__` dunders):

```c
// hash_key() - uses pointer address
return (uint64_t)(uintptr_t)key_ptr;

// keys_equal() - pointer comparison
return entry->key_ptr == key_ptr;

// dict_insert() - stores pointer directly (no copy)
new_entry->key_ptr = key_ptr;  // NOT owned
```

### Value-Based: With `__hash__`/`__eq__`

When class implements both dunders:

```c
// hash_key() - calls user function
return d->key_hash_fn(key_ptr);

// keys_equal() - calls user function
return d->key_eq_fn(entry->key_ptr, key_ptr);

// dict_insert() - copies key data
new_entry->key_ptr = malloc(d->key_size);
memcpy(new_entry->key_ptr, key_ptr, d->key_size);  // OWNED
```

## Enum Key Behavior

> [!IMPORTANT]
> **Why enum lookups fail with `Color.Red in dict`:**
>
> Each access to `Color.Red` creates a **new heap allocation** in the lowering phase:
> ```go
> // enum_lower.go: Color.Red becomes
> malloc(sizeof(Color)) → set tag = RED_TAG
> ```
>
> Since enums use pointer identity by default, different allocations = different pointers = no match.
>
> **Solution**: Store enum in variable first, use variable for both insert and lookup.

## Compiler Integration

### `lower_collections.go`

Detects key type and sets `TYPE_TAG_CUSTOM` for any class/struct/enum:

```go
switch kt := t.Key.(type) {
case *types.Class, *types.Struct, *types.Enum, *types.List, *types.Set:
    keyTypeTag = TYPE_TAG_CUSTOM
    if cls.Dunders["__hash__"] != nil {
        keyHashFn = @ClassName___hash__
    }
    if cls.Dunders["__eq__"] != nil {
        keyEqFn = @ClassName___eq__
    }
}
```

### Memory Management

| Key Type | Allocation | Ownership | Freed in `dict_free` |
|----------|------------|-----------|---------------------|
| `str` | `strdup()` | Owned | Yes |
| `int/bool/float` | Inline | N/A | No |
| Custom (default) | None (stores ptr) | NOT owned | No |
| Custom (dunders) | `malloc()` copy | Owned | Yes |

## Testing

| File | Tests |
|------|-------|
| `270_custom_type_dict_key.desi` | Class with `__hash__/__eq__` |
| `271_class_without_dunders_dict.desi` | Default pointer identity |
| `272_enum_dict_key.desi` | Enum as key (stored variable) |
| `273_nested_dict_custom_key.desi` | `dict[Point, dict[str, int]]` |
