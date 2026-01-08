# Closure Capture in TaskGroup

This document describes the internal implementation of closure capture for lambdas passed to `TaskGroup.run()`.

## Overview

When a lambda is passed to `tg.run()`, any outer variables it references are "captured" and passed to the spawned task. This requires special handling because:

1. The C runtime expects all spawned functions to have signature `fn(void* ctx)`
2. Lambda functions may have different signatures based on their captures
3. Captured values must be copied before the task runs (value-copy semantics)

## Architecture

```
User Code:              tg.run(lambda: print(x))
                              ↓
Lambda Desugaring:      __lam$0(x)  // x as parameter
                              ↓
Capture Marker:         __lam$0.__captures__(x)  // signals captures to lowering
                              ↓
Context Packing:        ctx = malloc(8); ctx[0] = x
                              ↓
Wrapper Generation:     __tgwrap$__lam$0(ctx) { x = ctx[0]; __lam$0(x) }
                              ↓
C Runtime Call:         taskgroup_spawn(tg, __tgwrap$__lam$0, ctx)
```

## Key Components

### 1. Capture Analysis (`capture_analysis.go`)

`CollectFreeVars(expr, scope)` recursively walks an expression and returns variable names that are:
- Used in the expression
- Not defined in the given scope
- Not built-in functions

### 2. Lambda Desugaring (`async_lambda.go`)

For each lambda:
1. Detect captured variables with `CollectFreeVars`
2. Add captures as extra parameters to the synthesized `__lam$N` function
3. Return `__lam$N.__captures__(a, b, c)` marker for lowering

### 3. Lowering (`lower_call.go`)

When lowering `tg.run()`:
1. Detect `__captures__` marker
2. Allocate context struct: `malloc(numCaptures * 8)`
3. Store each captured value at offset `i * 8`
4. Register wrapper via `RegisterTGWrapper(wrapperName, targetFn, numCaptures)`
5. Emit `taskgroup_spawn(tg, wrapperFunc, ctx)`

### 4. Wrapper Registry (`wrapper_registry.go`)

Global registry connecting lowering to LLVM backend:
- `RegisterTGWrapper(wrapperName, targetFn, numCaptures)` - called during lowering
- `GetTGWrappers()` - called by LLVM backend to emit wrappers

### 5. LLVM Backend (`module.go`)

`emitSingleWrapper` generates:
```llvm
define void @__tgwrap$__lam$0(ptr %__ctx__) {
entry:
  %cap0_ptr = getelementptr i8, ptr %__ctx__, i64 0
  %cap0 = load ptr, ptr %cap0_ptr
  call void @__lam$0(ptr %cap0)
  ret void
}
```

## Value-Copy Semantics

Captures use value-copy (like Rust's `move`):
- Values are copied into the context struct at `tg.run()` time
- The spawned task sees a snapshot of the values
- No sharing/aliasing issues with the parent task

## Primitive Boxing

Primitive types (int, float, bool, sized integers) need special handling because:
- Wrapper always loads captured values as `ptr` type
- Primitives stored directly would cause type mismatch and segfault

**Solution**: Box primitives before storing in context:

```
# For primitive capture 'x: int'
%boxed = call ptr @malloc(i64 8)    # Allocate box
store i32 42, ptr %boxed            # Store value in box
store ptr %boxed, ptr %ctx_slot     # Store box ptr in context
```

**Implementation** (lower_call.go):
1. `isPrimitiveType()` - detects int/float/bool/sized ints
2. Scope chain search - looks up variable types by name in `ls.scopes`
3. For primitives: malloc → store value → store ptr to context
4. For pointers (string, list, class): store directly

## Named Functions

For named functions without captures (`tg.run(simple_worker)`):
- Wrapper `__tgwrap$simple_worker(ctx)` is generated
- Wrapper ignores `ctx` and calls `simple_worker()`

## Send Trait Checking

Captured variables are checked at compile time to ensure they implement the Send trait:
- `check/check_send.go` contains `checkTaskGroupRunSend()`
- MutexGuard is NOT Send (lock ownership can't transfer across threads)
- Primitives, Mutex, Channel, Atomic are Send

## Future Work

- Reference capture mode (for intentional sharing with locks)
- Automatic context cleanup after task completion
