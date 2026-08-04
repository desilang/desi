# Full Async/Await Design (Future Implementation)

> **Status**: Deferred to Phase 5 (post M15)
> **Prerequisite**: Bound methods and TaskGroup working (Phase 4)

## Overview

This document describes the design for full async/await support in Desi, building on the structured concurrency foundation (TaskGroup, spawn).

## Current State

Desi uses **structured concurrency**:
- `spawn:` blocks for fire-and-forget tasks
- `TaskGroup.run(callable)` for managed task groups
- `using tg = sync.TaskGroup():` ensures all tasks complete

This covers ~90% of concurrent programming needs.

## Why Defer Full async/await?

1. **Complexity**: Full async requires suspension/resume (coroutines), stack management
2. **Runtime changes**: Needs async scheduler, continuation frames
3. **Borrow checker**: Must track borrows across await points
4. **Hot-reload**: Process isolation model changes semantics

## Future async/await Design

### Syntax (Proposed)

```desi
# Async function
async def fetch_data(url: str) -> str:
    let response = await http.get(url)  # Suspension point
    return response.body

# Calling async functions
def main() -> int:
    # Option 1: Explicit await
    let data = await fetch_data("https://example.com")
    
    # Option 2: spawn + await
    let task = spawn fetch_data("https://example.com")
    let data = await task
    
    # Option 3: TaskGroup with futures
    using tg = sync.TaskGroup():
        let tasks = [
            tg.spawn(fetch_data, "url1"),
            tg.spawn(fetch_data, "url2")
        ]
        let results = await tg.gather(tasks)
    
    return 0
```

### Runtime Requirements

1. **Continuation frames**: Save/restore local variables across await
2. **Async scheduler**: Cooperative scheduling of async tasks  
3. **Future type**: `Future[T]` for pending async results
4. **Cancellation**: Integration with TaskGroup.cancel()

### Memory Layout

```c
typedef struct {
    int state;           // Current suspension point
    void* result;        // Return value slot
    void* continuation;  // Next function to call
    char locals[];       // Variable-length saved locals
} AsyncFrame;
```

### Borrow Checker Integration

- **DBR0001**: No `inout` borrows across await (already defined)
- New: Track which locals are borrowed at suspension points
- Error if borrowed value could be invalidated during suspension

### Hot-Reload Compatibility

With process isolation (Elixir-style):
- Async tasks run in lightweight processes
- `await` = wait for message from worker process
- Hot-reload: old process finishes, new code handles new requests

### Implementation Steps (When Ready)

1. Add `async` flag to function AST (done in M1)
2. Transform async functions to state machines
3. Generate AsyncFrame allocation in constructor
4. Lower `await` to state save + return + resume
5. Add async scheduler to runtime
6. Integrate with TaskGroup.spawn() returning Future
7. Borrow checker: track borrows across suspension

## Related Documents

- [Sync Module](../../../docs/features/sync.md) - Current concurrency
- [Hot Reload Design](../../../docs/roadmap/planned/hot_reload_panic.md) - Process model
- [Roadmap M8](../../../docs/roadmap/roadmap.md) - Async milestone (current stub)

## Decision Points (For Future)

1. Should `async` be explicit (`async def`) or inferred?
2. Colored functions (async infection) or auto-blocking?
3. Single-threaded or multi-threaded async scheduler?
4. Stackless (state machine) or stackful (green threads)?

## See Also

- Rust async/await
- Python asyncio
- Elixir lightweight processes
- Go goroutines
