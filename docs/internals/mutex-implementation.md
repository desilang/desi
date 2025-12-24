# Mutex Implementation

This document explains the implementation of `Mutex[T]` in the Desi compiler, covering the type system, runtime, and cross-platform considerations.

## Architecture Overview

Mutex is implemented as a **hybrid** across three layers:

```
┌─────────────────────────────────────────────────────────────┐
│  Desi Source Code                                           │
│  let m = mutex_new(42)                                      │
│  let guard = m.lock()                                       │
│  print(guard.value)                                         │
└─────────────────────────────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Type Checker (Go)                                          │
│  - Infers Mutex[int] from argument type                     │
│  - Resolves lock() → MutexGuard[T]                          │
│  - Resolves guard.value → T                                 │
└─────────────────────────────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Lowering (Go → HIR)                                        │
│  - mutex_new() → malloc + store + call mutex_new(void*)     │
│  - lock() → call mutex_lock(DesiMutex*)                     │
│  - guard.value → call mutex_guard_get() + load              │
└─────────────────────────────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  C Runtime                                                  │
│  - POSIX: pthread_mutex_t                                   │
│  - Windows: SRWLOCK                                         │
└─────────────────────────────────────────────────────────────┘
```

## Type Definitions

### `compiler/internal/types/sync.go`

```go
type Mutex struct {
    Inner T // The type being protected
}

type MutexGuard struct {
    Inner T // The type being protected (for accessing the value)
}
```

Key design decisions:
- **Generic over T**: `Mutex[int]`, `Mutex[str]`, `Mutex[MyClass]` etc.
- **MutexGuard carries the inner type**: Enables `guard.value` to have the correct type.

## Type Checker Integration

### 1. `mutex_new()` Builtin

**File**: `check/info.go` (addPreludeBuiltins)
```go
addN("mutex_new",
    []types.T{nil}, // Any type - Mutex[T] inferred from argument
    []ast.ParamMode{ast.ParamMove},
    nil, // Mutex[T] - determined in expr_call.go
    []string{"value"},
)
```

**File**: `check/expr_call.go`
```go
if id.Name == "mutex_new" && len(args) == 1 && args[0] != nil {
    mutexType := types.MutexOf(args[0])
    c.info.Types[call] = mutexType
    return mutexType
}
```

### 2. `Mutex.lock()` Method

**File**: `check/expr_field.go`
```go
func (c *checker) resolveMutexMethod(x *ast.FieldExpr, m *types.Mutex) types.T {
    switch name {
    case "lock":
        // lock() -> MutexGuard[T]
        methodType = types.FuncOf(nil, types.MutexGuardOf(m.Inner), false)
    case "try_lock":
        // try_lock() -> Option[MutexGuard[T]]
        methodType = types.FuncOf(nil, types.OptionOf(types.MutexGuardOf(m.Inner)), false)
    }
}
```

### 3. `MutexGuard.value` Field

**File**: `check/expr_field.go`
```go
func (c *checker) resolveMutexGuardField(x *ast.FieldExpr, g *types.MutexGuard) types.T {
    if name == "value" {
        c.info.Types[x] = g.Inner
        return g.Inner
    }
}
```

## Lowering to LLVM IR

### Value Boxing

Primitive values must be **boxed** (heap-allocated) because the C runtime stores a `void*`:

**File**: `lower/lower_call.go`
```go
case "mutex_new":
    // Allocate memory for the value
    boxPtr := ls.b.FreshTemp("mutex_box")
    ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8"}}, Type: "ptr"})
    // Store the value into the box
    ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
    // Create mutex with pointer to boxed value
    ls.b.Emit(&hir.Call{Dst: res, Fn: "mutex_new", Args: []hir.Value{boxPtr}, Type: "ptr"})
```

### Value Unboxing

When accessing `guard.value`, we must **load** from the pointer:

**File**: `lower/lower_expr.go`
```go
if name == "value" {
    // Get pointer to boxed value
    ptrVal := ls.b.FreshTemp("guard_ptr")
    ls.b.Emit(&hir.Call{Dst: ptrVal, Fn: "mutex_guard_get", Args: []hir.Value{base}, Type: "ptr"})
    // Load the actual value
    loadedVal := ls.b.FreshTemp("guard_value")
    loadType := lowerType(g.Inner)
    ls.b.Emit(&hir.Load{Type: loadType, Src: ptrVal, Dst: loadedVal})
    return loadedVal
}
```

## C Runtime

### Platform Abstraction

**File**: `runtime/mutex.h`
```c
#ifdef _WIN32
    typedef SRWLOCK PlatformMutex;
#else
    typedef pthread_mutex_t PlatformMutex;
#endif

typedef struct {
    PlatformMutex lock;
    void* value;           /* Pointer to the protected value */
    bool initialized;
} DesiMutex;

typedef struct {
    DesiMutex* mutex;      /* The mutex we're holding */
    void* value;           /* Direct access to the value */
} MutexGuard;
```

### Key Functions

| Function | Description |
|----------|-------------|
| `mutex_new(void* value)` | Creates mutex, stores value pointer |
| `mutex_lock(DesiMutex*)` | Acquires lock, returns MutexGuard* |
| `mutex_try_lock(DesiMutex*)` | Non-blocking lock attempt |
| `mutex_guard_get(MutexGuard*)` | Returns pointer to protected value |
| `mutex_unlock(MutexGuard*)` | Releases lock, frees guard |

### Windows vs POSIX

| Operation | POSIX | Windows |
|-----------|-------|---------|
| Lock | `pthread_mutex_lock()` | `AcquireSRWLockExclusive()` |
| Unlock | `pthread_mutex_unlock()` | `ReleaseSRWLockExclusive()` |
| Try Lock | `pthread_mutex_trylock()` | `TryAcquireSRWLockExclusive()` |
| Destroy | `pthread_mutex_destroy()` | (no-op for SRWLOCK) |

## Testing

The comprehensive test covers:
- Primitive types: `int`, `float`, `bool`, `str`
- Edge cases: zero, negative numbers
- Custom types: `struct`, `class` (with `__new__`), `list`

**File**: `examples/214_mutex_test.desi`

## Known Limitations

1. **No automatic RAII cleanup**: Guard must be manually managed (future: defer integration)
2. **Boxing overhead**: Primitive values are heap-allocated
3. **No poisoning**: Panics inside critical section don't mark mutex as poisoned

## Related Files

- `compiler/internal/types/sync.go` - Type definitions
- `compiler/internal/check/expr_field.go` - Method/field resolution
- `compiler/internal/check/expr_call.go` - `mutex_new` builtin
- `compiler/internal/lower/lower_call.go` - Lowering for `mutex_new`, `.lock()`
- `compiler/internal/lower/lower_expr.go` - Lowering for `.value`
- `compiler/runtime/mutex.c` - C implementation
- `compiler/runtime/mutex.h` - C headers with Windows support
- `compiler/runtime/platform.h` - Platform abstraction macros
