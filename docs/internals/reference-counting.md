# Reference Counting (`Rc<T>`) - Internals

> **For Contributors**: Implementation details of Desi's reference counting mechanism.

---

## Overview

`Rc[T]` provides shared ownership of heap-allocated values through reference counting. When the count drops to zero, the value is freed.

## Runtime Functions

Located in `compiler/runtime/rc.c`:

| Function | Purpose |
|----------|---------|
| `__rc_new(ptr)` | Create Rc wrapper with refcount=1 |
| `__rc_inc(rc)` | Increment refcount |
| `__rc_dec(rc)` | Decrement refcount, free if zero |
| `__rc_clone(rc)` | Increment and return same pointer |
| `__rc_get(rc)` | Get inner pointer |
| `__rc_count(rc)` | Get current refcount (debugging) |

## Memory Layout

```
┌─────────────┐
│  refcount   │  8 bytes (size_t)
├─────────────┤
│  inner_ptr  │  8 bytes (void*)
└─────────────┘
```

## Compiler Integration

### Type Checker (`check/`)
- `info.go`: `rc` and `arc` added to built-in functions
- `expr_call.go`: Infers `Rc[T]` from `rc(value: T)`
- `expr_field.go`: Resolves `.get()` and `.clone()` methods

### Lowering (`lower/`)
- `lower_call.go`: `rc(x)` → box value, call `__rc_new`
- `lower_call.go`: `.get()` → `__rc_get` + `hir.Load`
- `lower_call.go`: `.clone()` → `__rc_clone`

### Backend (`llvm/`)
- `emit_func.go`: `hir.DecRef` → load pointer from alloca, call `__rc_dec`

## Usage Example

```python
let r := rc(42)        # Rc[int], refcount=1
let r2 := r.clone()    # refcount=2
let val := r.get()     # returns 42
# r and r2 go out of scope → refcount drops to 0, freed
```

## Future: `Arc<T>`

Atomic reference counting for thread-safe shared ownership. Same API as `Rc<T>` but uses atomic operations.
