# Raw Pointers (ptr)

Desi provides raw pointer types for C interoperability and low-level programming.

## Syntax

```desi
ptr[T]   # Typed pointer to T (e.g., ptr[u8], ptr[i32])
ptr      # Opaque pointer (equivalent to C's void*)
```

## Why Raw Pointers?

Raw pointers are needed when:
- Calling C libraries (malloc, file handles, etc.)
- Interfacing with system APIs
- Performance-critical code requiring direct memory access

## Basic Usage

### Declaring Extern Functions

```desi
@extern
def malloc(size: int) -> ptr      # Returns opaque pointer

@extern
def get_buffer() -> ptr[u8]       # Returns typed pointer to bytes
```

### Using Pointers with Unsafe Blocks

All pointer operations require an `unsafe:` block:

```desi
def allocate_buffer(size: int) -> ptr:
    unsafe:
        let buffer = malloc(size)
        return buffer
```

## Memory Safety

> **Important**: Raw pointers bypass Desi's memory safety guarantees. Use them carefully.

### Non-Nullable by Default

`ptr` and `ptr[T]` are **never null**. For nullable pointers, use `Option`:

```desi
@extern
def find_resource(name: str) -> Option[ptr]

def use_resource(name: str):
    if find_resource(name) is Some(p):
        unsafe:
            # p is guaranteed non-null here
            use_ptr(p)
    else:
        print("Resource not found")
```

### Unsafe Blocks

The `unsafe:` block signals to readers and the compiler that you're taking responsibility for memory safety:

```desi
def process_data():
    unsafe:
        # All pointer operations go here
        let buf = malloc(1024)
        process_buffer(buf)
        free(buf)
```

## Common Patterns

### C FFI

```desi
@extern
def fopen(path: ptr[i8], mode: ptr[i8]) -> ptr
@extern  
def fclose(f: ptr) -> int
@extern
def fread(buf: ptr[u8], size: int, n: int, f: ptr) -> int
```

### Passing Pointers to Functions

```desi
def use_buffer(buf: ptr[u8], len: int) -> int:
    # Work with the buffer
    0

def main() -> int:
    unsafe:
        let buf = malloc(1024)
        use_buffer(buf, 1024)
    0
```

## Quick Reference

| Type | Description | Example |
|------|-------------|---------|
| `ptr[T]` | Typed pointer | `ptr[u8]`, `ptr[i32]` |
| `ptr` | Opaque pointer (void*) | `ptr` |
| `Option[ptr[T]]` | Nullable pointer | `Option[ptr[u8]]` |

## Best Practices

1. **Minimize unsafe code** - Keep `unsafe:` blocks small and focused
2. **Use typed pointers** - Prefer `ptr[T]` over `ptr` when the type is known
3. **Handle null properly** - Use `Option[ptr]` for potentially null pointers
4. **Document unsafe code** - Explain why unsafe is needed and what invariants must hold
