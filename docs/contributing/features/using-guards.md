# RAII Using Guards

This document explains how `using` guards implement RAII cleanup for various types.

## Overview

The `using` statement ensures resources are cleaned up at scope exit:

```desi
using tx = ch.sender():
    using rx = ch.receiver():
        tx.send(42)
        # Both tx and rx cleaned up here
```

## Supported Types

| Type | Cleanup Function |
|------|------------------|
| Arena | `__arena_destroy` |
| File | `file_close` |
| MutexGuard | `mutex_unlock` |
| ReadGuard | `read_guard_unlock` |
| WriteGuard | `write_guard_unlock` |
| Sender | `sender_drop` |
| Receiver | `receiver_drop` |
| Class with `__close__` | `ClassName___close__` |

## Implementation

### Registration (lower_stmt.go)

At `using` statement, type is checked and registered in scope:

```go
if isSenderType(typ) {
    ls.cur().senders[ident] = true
} else if isReceiverType(typ) {
    ls.cur().receivers[ident] = true
}
// ... etc
```

### Cleanup (hir_lower.go)

`emitScopeDrops()` emits cleanup calls in reverse order:

```go
if sc.senders[varName.Name] {
    ls.b.Emit(&hir.Call{Fn: "sender_drop", Args: []hir.Value{v}})
}
```

## Files

| File | Purpose |
|------|---------|
| `file_lower.go` | Type check helpers (`isSenderType`, etc.) |
| `hir_lower.go` | Scope struct, `emitScopeDrops()` cleanup |
| `lower_stmt.go` | UsingStmt registration logic |
