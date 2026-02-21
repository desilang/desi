# Async/Futures Implementation

This document explains the implementation of `async def` and `await` in the Desi compiler.

## Architecture Overview

Desi uses a **thread-based future model**. Each `async def` spawns its body on a background pthread and returns a `Future` handle immediately. `await` blocks the caller until the future completes.

```
┌── main thread ──────────────────────────────────────────────┐
│  fut = __future_new()                                       │
│  __future_spawn_1(fut, add1$body, x)   ──┐                 │
│  return fut                              │                  │
│  ...                                     │                  │
│  result = __await_blocking(fut)  ◀───────┤─── completes     │
└──────────────────────────────────────────┘                  │
                                                              │
┌── background thread ─────────────────────┐                  │
│  val = add1$body(fut, x)                 │                  │
│  __future_complete(fut, val)  ───────────┘                  │
└──────────────────────────────────────────┘
```

### Compilation Model

An `async def` is split into two HIR functions:

1. **Wrapper** (`f`): Creates future → spawns body → returns future handle (`ptr`)
2. **Body** (`f$body`): Receives `(future_ptr, original_params...)` → runs user code → returns `i64` (widened)

## Type Transport (i64 Channel)

All values pass through the C runtime as `int64_t` to avoid needing generic/templated C code:

| Conversion Point | Direction | Mechanism |
|-----------------|-----------|-----------|
| Spawn args | native → i64 | `sext i32→i64`, `ptrtoint ptr→i64` |
| Body return | native → i64 | auto-widen in `emitRet` (sext/ptrtoint/zext/bitcast) |
| Await result | i64 → native | `trunc i64→i32`, `inttoptr i64→ptr`, `bitcast i64→double` |

### Why i64?

- Fits `int` (i32, sign-extended), `bool` (i1), `ptr` (64-bit), `double` (bitcast)
- Avoids C varargs/generics complexity
- Matches pthreads' `void*` size on 64-bit platforms

## HIR Nodes

**File**: `hir/async_nodes.go`

| Node | Purpose |
|------|---------|
| `FutureNew{Dst}` | Allocate a future handle |
| `FutureSpawn{Fut, BodyFn, Args}` | Spawn body on background thread |
| `Await{Fut, Dst, ResultType}` | Block until complete, narrow result |
| `FutureComplete{Fut, Val}` | Complete a future (used by body internally) |

`FutureSpawn` is a dedicated node (not generic `Call`) so the LLVM backend can emit correctly-typed `ptr`/`i64` arguments without relying on the generic call emitter.

`Await.ResultType` tells the LLVM backend what type to narrow the `i64` to (e.g., `"i32"`, `"ptr"`, `"double"`).

## Async Lowering

**File**: `lower/async_lower.go`

### `LowerAsyncFunc(fd, src, info, globalNames)`

Returns `(wrapper *hir.Func, body *hir.Func)`.

```go
// 1. Create wrapper with same params as original
wb := hir.NewFunc(name)
wb.Params = fd.Params  // copied with types from body

// 2. Emit: fut = future.new; future.spawn(fut, body, args...); ret fut
wb.Emit(&hir.FutureNew{Dst: fut})
wb.Emit(&hir.FutureSpawn{Fut: fut, BodyFn: name+"$body", Args: params})
wb.Emit(&hir.Ret{Val: fut})

// 3. Lower body normally, prepend __future__ param, set RetType = "i64"
bodyFunc := LowerFuncFromDeclEx(fd, ...)
bodyFunc.Name = name + "$body"
bodyFunc.Params = prepend({Name: "__future__", Type: "ptr"}, bodyFunc.Params)
bodyFunc.RetType = "i64"
```

### `desiTypeToLLVM(t types.T) string`

Maps Desi type checker types to LLVM IR type strings:

- `int` → `"i32"`, `bool` → `"i1"`, `str` → `"ptr"`, `float` → `"double"`
- Structs, classes, lists, dicts, enums, tuples, sets, futures → `"ptr"`

## LLVM Backend

### Async Wrapper Detection

**File**: `cmd/desic/emit_ir_cmd.go` → `markAsyncWrappers()`

Scans HIR for `<name>$body` functions and marks `<name>` as an async wrapper. This makes the wrapper return `ptr` instead of the default `i32`.

### Function Signature Overrides

**File**: `backend/llvm/sig_overrides.go`

```go
SetFuncSig("__future_new", "ptr", nil)
SetFuncSig("__future_complete", "void", []string{"ptr", "i64"})
SetFuncSig("__await_blocking", "i64", []string{"ptr"})
SetFuncSig("__future_spawn_0", "void", []string{"ptr", "ptr"})
SetFuncSig("__future_spawn_1", "void", []string{"ptr", "ptr", "i64"})
// ... up to spawn_4
```

### FutureSpawn Emission

**File**: `backend/llvm/emit_func.go`

Args are widened to i64 before calling `__future_spawn_N`:
- `i32` → `sext i32 %x to i64`
- `ptr` → `ptrtoint ptr %x to i64`

### Await Emission

After `__await_blocking` returns `i64`, narrows based on `Await.ResultType`:
- `"i32"` → `trunc i64 %raw to i32`
- `"ptr"` → `inttoptr i64 %raw to ptr`
- `"double"` → `bitcast i64 %raw to double`
- `"i1"` → `trunc i64 %raw to i1`

### Body Return Widening

**File**: `backend/llvm/module.go` → `emitRet()`

When `curFuncRetTy == "i64"` but the operand is narrower:
- `i32` → `sext i32 %val to i64; ret i64 %tmp`
- `ptr` → `ptrtoint ptr %val to i64; ret i64 %tmp`
- `double` → `bitcast double %val to i64; ret i64 %tmp`

## C Runtime

**File**: `runtime/future.c`

### DesiFuture Struct

```c
typedef struct DesiFuture {
    pthread_mutex_t lock;
    pthread_cond_t  done_cv;
    int64_t         value;
    int             completed;
    int             thread_started;
    pthread_t       thread;
} DesiFuture;
```

### Key Functions

| Function | Description |
|----------|-------------|
| `__future_new()` | Allocate + init mutex/condvar |
| `__future_complete(fut, val)` | Set value, signal condvar |
| `__await_blocking(fut)` | Wait on condvar, join thread, free resources |
| `__future_spawn_N(fut, body, args...)` | Pack ctx, `pthread_create` |

### Thread Entry

`__future_thread_entry` unpacks `FutureSpawnCtx`, dispatches body by arity (switch on argc), and calls `__future_complete` with the result.

## Edge Cases and Gotchas

1. **Wrapper params default to `ptr`** in the LLVM backend unless explicitly typed. The lowerer copies types from the body to avoid this.

2. **Body `__future__` param** is always `ptr` — never used by user code currently, but available for future `__future_complete` calls from within the body.

3. **No `inout` across await** — enforced by `CheckAwaitBorrowBarrier()` in the lowerer. Compile-time error.

4. **Thread safety**: Each future has its own mutex/condvar. No global lock contention.

5. **Memory**: Future is heap-allocated, freed by `__await_blocking`. If a future is never awaited, it leaks. (Future work: destructor/GC integration.)

## Related Files

- `compiler/internal/hir/async_nodes.go` — HIR node definitions
- `compiler/internal/hir/print.go` — HIR printer (FutureSpawn case)
- `compiler/internal/lower/async_lower.go` — Async function lowering
- `compiler/internal/lower/lower_expr.go` — Await expression lowering
- `compiler/internal/lower/await_barrier.go` — Borrow barrier checking
- `compiler/internal/backend/llvm/emit_func.go` — LLVM emission
- `compiler/internal/backend/llvm/module.go` — emitRet widening
- `compiler/internal/backend/llvm/sig_overrides.go` — Function signatures
- `compiler/cmd/desic/emit_ir_cmd.go` — markAsyncWrappers
- `compiler/runtime/future.c` — C runtime
