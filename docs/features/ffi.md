# FFI (Foreign Function Interface)

Desi supports calling C libraries through its FFI system, with safety features to prevent common bugs.

## Extern Declarations

Declare C functions with `@extern`:

```desi
@extern("C")
def strlen(s: cptr) -> int

@extern("C")
def printf(fmt: cptr, ...) -> int
```

## unsafe Blocks

FFI calls must be wrapped in `unsafe:` blocks:

```desi
def main() -> int:
    unsafe:
        let len = strlen("hello".as_cptr())
        printf("Length: %d\n".as_cptr(), len)
    0
```

Calling an extern function outside `unsafe:` is a compile error (DFI0003).

## Safe Wrappers

Auto-generate safe wrappers with `safe=true`:

```desi
@extern("C", safe=true, c_name="abs")
pub def absolute(x: int) -> int

# Now you can call it without unsafe:
print(absolute(-42))  # prints 42
```

The compiler generates:
1. A raw extern using the C name
2. A public wrapper that calls it inside `unsafe:`

## C-Compatible Structs

Use `@ffi_struct` for C-compatible memory layout:

```desi
@ffi_struct
struct Point:
    x: i32
    y: i32

@ffi_struct
struct Rect:
    origin: Point
    size: Point
```

**Restrictions:**
- Only FFI-compatible types allowed (primitives, `cptr`, other `@ffi_struct`)
- Cannot be generic
- Error DFI0004 for invalid field types
- Error DFI0005 for generic `@ffi_struct`

## FFI-Compatible Types

| Desi Type | C Type |
|-----------|--------|
| `i8`, `i16`, `i32`, `i64` | `int8_t`, `int16_t`, etc. |
| `u8`, `u16`, `u32`, `u64` | `uint8_t`, `uint16_t`, etc. |
| `f32`, `f64` | `float`, `double` |
| `bool` | `bool` / `_Bool` |
| `cptr` | `void*` |
| `@ffi_struct` types | corresponding C struct |

## Related

- [Arena Memory](arena.md) - Allocate memory for FFI
- [Sync Primitives](sync.md) - Thread-safe FFI usage
