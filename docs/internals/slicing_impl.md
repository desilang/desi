# Slicing and Indexing Implementation

This document describes Desi's slicing and indexing implementation for contributors.

## Overview

Desi supports Python-style slicing for lists and strings with negative indices and optional step parameter.

## Syntax

```desi
list[start:end]        # Basic slice
list[start:end:step]   # Step slicing
list[-1]               # Negative indexing (last element)
list[::-1]             # Reverse (full slice with step -1)
```

## Implementation Details

### Negative Indexing

**Location:** `compiler/internal/lower/lower_expr.go`

Negative indices are converted to positive by the runtime functions. The lowering emits calls to:
- `list_get(list, index)` - for single element access
- `list_slice(list, start, end)` - for basic slicing
- `list_slice_step(list, start, end, step)` - for step slicing

**Runtime:** `compiler/runtime/list.c`

```c
// Negative index handling in list_get
if (index < 0) {
    index = len + index;  // -1 becomes len-1
}
```

### Step Slicing

**Location:** `compiler/internal/lower/lower_expr.go` (lines 394-418)

When a step parameter is present (`SliceExpr.K != nil`):
1. Use sentinel values for omitted bounds:
   - `INT64_MAX` for omitted start
   - `INT64_MIN` for omitted end
2. Call `list_slice_step` or `string_slice_step`

**Runtime Sentinel Handling:** `compiler/runtime/list.c`

```c
#define SENTINEL_START 9223372036854775807LL
#define SENTINEL_END   (-9223372036854775807LL - 1)

// Forward step (step > 0): start=0, end=len
if (step > 0) {
    if (start == SENTINEL_START) start = 0;
    if (end == SENTINEL_END) end = len;
}
// Reverse step (step < 0): start=len-1, end=-1
else {
    if (start == SENTINEL_START) start = len - 1;
    if (end == SENTINEL_END) end = -1;
}
```

### String Slicing

String slicing follows the same pattern but uses separate runtime functions:
- `string_slice(str, start, end)` 
- `string_slice_step(str, start, end, step)`

## Custom Classes (`__getitem__`)

Classes can implement `__getitem__` to support indexing:

```desi
class MyList:
    pub data: list[int]
    
    pub def __getitem__(self, index: int) -> int:
        return self.data[index]
```

**Lowering:** Desugars `obj[idx]` to `ClassName___getitem__(obj, idx)`

## Tests

- `examples/87_list_slicing.desi` - Basic list slicing
- `examples/204_negative_indexing.desi` - Negative indices
- `examples/205_step_slice.desi` - Step slicing `[::2]`, `[::-1]`

## Commits

- `1b63297` - Negative indexing for lists and strings
- `4170a78` - Step slicing with sentinel values
