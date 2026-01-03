# Raw Pointers: ptr and ptr[T]

This document covers the design rationale and implementation details of raw pointer types in Desi.

## Design Rationale

### Why Raw Pointers?

Raw pointers are necessary for:
1. **C FFI** - Interfacing with C libraries (malloc, file handles, etc.)
2. **Performance** - Zero-cost abstraction for low-level memory operations
3. **System programming** - Memory-mapped I/O, direct memory manipulation

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| `ptr[T]` not `*T` | Python-like syntax, consistent with `list[T]`, `dict[K,V]` |
| `ptr` = opaque | Shorthand for `ptr[none]`, equivalent to C's `void*` |
| Non-nullable | Use `Option[ptr[T]]` for nullable, eliminates null bugs |
| Alias for `cptr` | `ptr` is more intuitive than `cptr` for most users |

### Memory Safety

- All pointer operations should occur in `unsafe:` blocks
- Compiler doesn't track pointer validity (unlike safe references)
- User is responsible for:
  - Not dereferencing freed memory
  - Ensuring pointer alignment
  - Proper lifetime management

## Implementation

### Type System (`types/types.go`)

```go
// CPtr is the underlying type (historical name)
type CPtr struct{ Elem T }

func (t *CPtr) String() string { return "cptr[" + t.Elem.String() + "]" }
func CPtrOf(elem T) *CPtr { return &CPtr{Elem: elem} }

// FromName handles "ptr" and "cptr" as opaque pointers
case "ptr", "cptr":
    return CPtrOf(None), true
```

### Type Resolution (`check/typename_resolver.go`)

```go
case "cptr", "ptr":
    // ptr[T] or cptr[T] - typed pointer
    if len(tn.Params) != 1 {
        return nil
    }
    elem := c.resolveType(tn.Params[0])
    return types.CPtrOf(elem)
```

### LLVM Lowering

All pointer types lower to LLVM's opaque `ptr` type:
```llvm
declare ptr @malloc(i64)
define ptr @get_buffer() { ... }
```

## Usage Examples

### C FFI
```desi
@extern
def malloc(size: int) -> ptr
@extern
def free(p: ptr)

def allocate(n: int) -> ptr:
    unsafe:
        return malloc(n)
```

### Typed Pointers
```desi
@extern
def get_buffer() -> ptr[u8]

def process(buf: ptr[u8], len: int):
    unsafe:
        # Work with typed buffer
        pass
```

### Nullable Pointers
```desi
@extern
def find_resource(name: str) -> Option[ptr]

def use_resource(name: str):
    if find_resource(name) is Some(p):
        unsafe:
            # Use p
            pass
```

## Common Pitfalls

1. **Forgetting unsafe** - Pointer operations need `unsafe:` block
2. **Null confusion** - `ptr` is non-nullable, use `Option[ptr]` for nullable
3. **Type mismatch** - `ptr` (opaque) and `ptr[T]` (typed) are different types
