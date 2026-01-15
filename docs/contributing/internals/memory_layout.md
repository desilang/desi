# Memory Layout & Alignment

## Overview
Desi follows the x86-64 System V ABI for data structure alignment to ensure memory safety and optimal performance.

## Alignment Requirements

All fields in classes, structs, and enums must be properly aligned according to their type:

| Type | Size (bytes) | Alignment (bytes) |
|------|--------------|-------------------|
| `bool`, `i8`, `u8` | 1 | 1 |
| `i16`, `u16` | 2 | 2 |
| `int`, `i32`, `u32`, `f32` | 4 | 4 |
| `i64`, `u64`, `float`, `f64` | 8 | 8 |
| `str`, pointers, references | 8 | 8 |

## Data Structure Layouts

### Classes

Fields are laid out sequentially with padding added before each field to meet its alignment requirement:

```desi
class Example:
    pub a: int      # 4 bytes at offset 0
    pub b: str      # 8 bytes at offset 8 (4 bytes padding added)
    # Total: 16 bytes
```

**Memory Layout:**
```
[a:i32][padding:4][b:ptr]
0      4          8       16
```

### Enums

All enums use a uniform layout regardless of variant:
- **Tag**: `i32` (4 bytes) at offset 0
- **Padding**: 4 bytes
- **Payload**: `ptr` (8 bytes) at offset 8
- **Total**: 16 bytes

```desi
enum Status:
    Idle: none
    Running: int
    Error: str
```

**Memory Layout (all variants):**
```
[tag:i32][padding:4][payload_ptr:ptr]
0        4          8                16
```

The payload pointer points to heap-allocated storage for the variant's data.

### Structs

Like classes, struct fields are aligned according to their types:

```desi
struct Worker:
    id: int         # 4 bytes at offset 0
    status: Status  # 8 bytes at offset 8 (enum is a pointer)
    # Total: 16 bytes
```

## Implementation

### Size Calculation

The compiler uses `getClassSize()`, which calculates the total size with proper padding:

```go
func getClassSize(fields []types.Field) int {
    offset := 0
    for _, field := range fields {
        fieldSize := getSize(field.Type)
        fieldAlign := getAlign(field.Type)
        
        // Align offset to field's alignment
        if fieldAlign > 0 && offset%fieldAlign != 0 {
            offset += fieldAlign - (offset % fieldAlign)
        }
        offset += fieldSize
    }
    return offset
}
```

### Field Access

When accessing fields, the compiler calculates the aligned offset:

```go
offset := 0
for each field before target {
    offset = align(offset, field.alignment)
    if field == target: break
    offset += field.size
}
```

## Why Alignment Matters

### Memory Safety
- **Unaligned access** can cause undefined behavior or segfaults
- On x86-64, unaligned pointer access may work but is slower
- On ARM and other architectures, unaligned access causes crashes

### Performance
- **Aligned access** uses optimized CPU instructions
- Cache lines are loaded efficiently
- SIMD operations require aligned data

### Trade-offs
- **Space**: Padding adds overhead (typically 0-7 bytes per field)
- **Safety**: Prevents crashes and undefined behavior ✅
- **Performance**: Optimal memory access patterns ✅

## Examples

### Multi-Parameter Generics

```desi
class Pair<T, U>:
    pub first: T
    pub second: U
```

Instantiations:
- `Pair<int, int>`: `[first:4][second:4]` = 8 bytes (both 4-byte aligned)
- `Pair<int, str>`: `[first:4][pad:4][second:8]` = 16 bytes (str needs 8-byte alignment)
- `Pair<str, str>`: `[first:8][second:8]` = 16 bytes (both 8-byte aligned)

### Enum in Struct

```desi
enum Status:
    Error: str

struct Worker:
    id: int        # offset 0
    status: Status # offset 8 (aligned)
```

The `Status` enum itself is always 16 bytes, and the `Worker` struct stores a pointer to it.

## Enum Drop Cleanup

When an enum goes out of scope, the compiler generates cleanup code to free heap-allocated payloads:

1. **Load tag** from offset 0
2. **Switch on tag** to determine variant
3. For variants with heap fields:
   - Load payload pointer from offset 8
   - Recursively drop each heap-allocated field
   - Free the payload struct
4. **Free the enum** wrapper (16 bytes)

This prevents memory leaks for enums with payloads like `str`, `list`, `dict`, or nested enums.

**Implementation**: [`drop_impl.go:emitEnumDrop()`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/backend/llvm/drop_impl.go)

## Related Files

- [`compiler/internal/lower/module_lower.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/module_lower.go) - `getAlign()`, `getClassSize()`, struct constructors
- [`compiler/internal/lower/class_lower.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/class_lower.go) - Class allocation with alignment
- [`compiler/internal/lower/enum_lower.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/enum_lower.go) - Enum layout (16 bytes)
- [`compiler/internal/lower/lower_expr.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/lower_expr.go) - Field access offset calculation
- [`compiler/internal/backend/llvm/drop_impl.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/backend/llvm/drop_impl.go) - Drop cleanup (struct, enum, list, dict, set)

## Historical Context

Prior to January 2026, the compiler calculated field offsets by simply summing field sizes without padding. This worked for same-sized fields but caused segfaults when mixing 4-byte and 8-byte fields. The alignment fixes ensure memory safety across all type combinations.
