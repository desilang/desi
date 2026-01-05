# TaskGroup Implementation

This document explains the implementation of `TaskGroup` in the Desi compiler.

## Architecture Overview

TaskGroup provides structured concurrency - all spawned tasks must complete before the group exits:

```
┌─────────────────────────────────────────────────────────────┐
│  let group = taskgroup_new()                                │
│  group.spawn(worker1)                                       │
│  group.spawn(worker2)                                       │
│  group.spawn(worker3)                                       │
│  group.wait()   // Blocks until all workers complete        │
└─────────────────────────────────────────────────────────────┘
```

## Type Definition

### `compiler/internal/types/taskgroup.go`

```go
type TaskGroup struct {
    // TaskGroup doesn't hold a type parameter
}
```

TaskGroup is not generic - it manages heterogeneous tasks.

## Type Checker Integration

### 1. `taskgroup_new()` Builtin

**File**: `check/info.go`
```go
addN("taskgroup_new",
    []types.T{}, // No arguments
    []ast.ParamMode{},
    types.TaskGroupOf(),
    []string{},
)
```

### 2. TaskGroup Methods

**File**: `check/expr_field.go`
```go
func (c *checker) resolveTaskGroupMethod(x *ast.FieldExpr) types.T {
    switch name {
    case "spawn":
        return types.FuncOf([]types.T{types.Any}, types.None, false)
    case "wait":
        return types.FuncOf(nil, types.None, false)
    case "cancel":
        return types.FuncOf(nil, types.None, false)
    case "is_cancelled":
        return types.FuncOf(nil, types.Bool, false)
    }
}
```

## Lowering to LLVM IR

**File**: `lower/lower_call.go`

### taskgroup_new

```go
if calleeName == "taskgroup_new" && len(x.Args) == 0 {
    res := ls.b.FreshTemp("taskgroup")
    ls.b.Emit(&hir.Call{Dst: res, Fn: "taskgroup_new", Args: []hir.Value{}, Type: "ptr"})
    return res
}
```

### Method calls

```go
case "wait":
    ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_wait", Args: []hir.Value{groupVal}, Type: "void"})
case "cancel":
    ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_cancel", Args: []hir.Value{groupVal}, Type: "void"})
case "is_cancelled":
    ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_is_cancelled", Args: []hir.Value{groupVal}, Type: "i1"})
```

## C Runtime

### Platform Abstraction

**File**: `runtime/taskgroup.h`
```c
typedef struct TaskGroup {
    Task** tasks;               
    int64_t capacity, count;    
    int64_t pending;            
    
    DesiPlatformMutex lock;     
    DesiPlatformCond all_done;  
    
    bool cancelled;             
    void* error;                
} TaskGroup;
```

### Key Functions

| Function | Description |
|----------|-------------|
| `taskgroup_new()` | Create new group |
| `taskgroup_spawn(g, fn, ctx)` | Spawn task in group |
| `taskgroup_wait(g)` | Block until all complete |
| `taskgroup_cancel(g)` | Mark as cancelled |
| `taskgroup_is_cancelled(g)` | Check cancellation |
| `taskgroup_destroy(g)` | Free resources |

## Related Files

- `compiler/internal/types/taskgroup.go`
- `compiler/internal/check/expr_field.go`
- `compiler/internal/lower/lower_call.go`
- `compiler/runtime/taskgroup.c`
- `compiler/runtime/taskgroup.h`
