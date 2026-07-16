# Reference Counting (`Rc<T>`) - Internals

> **For Contributors**: Implementation details of Desi's reference counting mechanism.

---

## Overview

`Rc[T]` provides shared ownership of heap-allocated values through reference counting. When the count drops to zero, the value is freed.

## Runtime Functions

Located in `compiler/runtime/rc.c`:

| Function | Purpose |
|----------|---------|
| `__rc_new(ptr)` | Create Rc wrapper with refcount=1, weakcount=0 |
| `__rc_inc(rc)` | Increment refcount |
| `__rc_dec(rc)` | Decrement refcount, free inner if 0, free header if weakcount=0 |
| `__rc_clone(rc)` | Increment and return same pointer |
| `__rc_get(rc)` | Get inner pointer |
| `__rc_count(rc)` | Get current refcount (debugging) |
| `__weak_new(rc)` | Create Weak pointer (increments weakcount) |
| `__weak_dec(rc)` | Decrement weakcount, free header if refcount=0 & weakcount=0 |
| `__weak_upgrade(rc)` | Upgrade weak to strong, returns rc_ptr or NULL |
| `__weak_count(rc)` | Get current weakcount (debugging) |

## Memory Layout

```
┌─────────────┐
│  refcount   │  4 bytes (int32_t)
├─────────────┤
│  weakcount  │  4 bytes (int32_t)
├─────────────┤
│  inner_ptr  │  8 bytes (void*)
└─────────────┘
```

The control block (header) remains allocated as long as `refcount > 0` or `weakcount > 0`. When `refcount` hits 0, `inner_ptr` is freed immediately and set to `NULL`, but the control block itself is only freed when `weakcount` also reaches 0.

## Compiler Integration

### Type Checker (`check/`)
- `expr_call.go`: Infers `Rc[T]` from `rc(value)` and `Weak[T]` from `weak(rc_value)`.
- `typename_resolver.go`: Resolves parameterized `rc[T]`, `arc[T]`, and `weak[T]` annotations.
- `expr_field.go`: Resolves `.get()` and `.clone()` methods on `Rc[T]` and `.upgrade()` on `Weak[T]` (which returns `Option[rc[T]]`).

### Lowering (`lower/`)
- `lower_call.go`:
  - `rc(x)` → malloc payload box, call `__rc_new`
  - `weak(rc)` → call `__weak_new`
  - `w.upgrade()` → call `__weak_upgrade`, branch on null, and call `Option.Some(ptr)` or `Option.Nothing()` to return a heap-allocated Option.
- `hir_lower.go`: Tracks variables of type `Rc[T]`/`Arc[T]` in `rcLike` and `Weak[T]` in `weakLike`.

### Backend (`llvm/`)
- `drop_impl.go`: 
  - Generates `__rc_dec` call for `Rc` / `Arc` drops.
  - Generates `__weak_dec` call for `Weak` drops.
  - Recursively drops nested smart pointer fields inside structs and enums (like `Option`).

## Usage Example

```desi
let r = rc(42)          # rc[int], refcount=1, weakcount=0
let w = weak(r)         # weak[int], refcount=1, weakcount=1

match w.upgrade():
    Some(strong):
        print(strong.get()) # 42
    Nothing:
        print("dead")
```
