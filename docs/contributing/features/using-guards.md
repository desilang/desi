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

## Guard Escape Analysis

Guards are non-escaping types that must not leave their scope. The type checker detects escape attempts.

### Diagnostic Codes

| Code | Description |
|------|-------------|
| DSY0002 | Returning a guard type from a function |
| DSY0003 | Returning an arena-allocated value (planned) |
| DSY0004 | Storing a guard in a global variable (planned) |
| DSY0005 | Passing a guard to a spawn function (planned) |

### Implementation (stmt.go)

```go
// isNonEscapingType returns true if t is a type that must not escape its scope.
func (c *checker) isNonEscapingType(t types.T) bool {
    switch t.(type) {
    case *types.MutexGuard, *types.ReadGuard, *types.WriteGuard:
        return true
    }
    return false
}
```

Return statement check:

```go
case *ast.ReturnStmt:
    rt := c.typ(st.Value)
    if c.isNonEscapingType(rt) {
        c.add(diagAt("DSY0002", st.Span,
            "cannot return guard type '"+rt.String()+"' - guards must not escape their scope"))
    }
```

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

## LLVM Backend: Conditionals in Using Blocks

When `if` statements appear inside `using` blocks, the LLVM backend must handle control flow correctly.

### The Bug (Fixed Jan 2025)

**Symptom**: Infinite loops or LLVM errors when conditionals used inside `using` blocks.

**Root Cause**: After emitting a conditional branch, the merge block label was emitted but:
1. `lifetime.end` calls were emitted after the branch (unreachable code)
2. Empty merge blocks (both branches return) had no terminator

**Fix** (emit_func.go):
1. Set `lifetimesClosed = true` after emitting If branch
2. Detect empty merge blocks and emit `unreachable` terminator

```go
case *hir.If:
    // ... emit branch ...
    wprintf(&m.funcs, "%s:\n", mergeLabel)
    lifetimesClosed = true  // Prevent unreachable lifetime.end

// After statement loop
if _, isIf := b.Stmts[len(b.Stmts)-1].(*hir.If); isIf {
    if lifetimesClosed && m.cfBlocks[label] == "" {
        wprintf(&m.funcs, "  unreachable\n")
    }
}
```

## Files

| File | Purpose |
|------|---------|
| `file_lower.go` | Type check helpers (`isSenderType`, etc.) |
| `hir_lower.go` | Scope struct, `emitScopeDrops()` cleanup |
| `lower_stmt.go` | UsingStmt registration logic |
| `check/stmt.go` | Guard escape analysis (`isNonEscapingType`) |
| `backend/llvm/emit_func.go` | LLVM IR generation for control flow |

