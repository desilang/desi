# Send & Sync Traits - Implementation Guide

## Overview

The Send and Sync traits provide compile-time thread-safety guarantees for Desi's concurrency primitives.

- **Send**: A type is Send if ownership can be safely transferred across task boundaries
- **Sync**: A type is Sync if references can be safely shared across tasks

## Implementation

### Core Functions (`types/send_sync.go`)

```go
// IsSend returns true if type can be safely sent across tasks
func IsSend(t T) bool

// IsSync returns true if references can be safely shared
func IsSync(t T) bool

// IsSendSync returns true if both Send and Sync
func IsSendSync(t T) bool
```

### Type Classifications

| Type | Send | Sync | Reason |
|------|------|------|--------|
| Primitives (int, str, bool, float) | ✓ | ✓ | Immutable/Copy semantics |
| List, Set, Dict | ✓* | ✓* | *If elements are Send/Sync |
| Tuple | ✓* | ✓* | *If all elements are Send/Sync |
| Channel | ✓ | ✓ | Designed for cross-task use |
| Mutex | ✓ | ✓ | Thread-safe by design |
| **MutexGuard** | ✗ | ✗ | Holds lock, must not cross tasks |
| Rc | ✗ | ✗ | Non-atomic refcount |
| Arc | ✓ | ✓ | Atomic refcount |
| CPtr | ✗ | ✗ | Raw pointer, unsafe |

### Spawn Statement Checking

The type checker verifies Send safety in spawn blocks (`check/stmt.go`):

```go
case *ast.SpawnStmt:
    captured := c.collectCapturedVariables(st.Body)
    for varName, sym := range captured {
        if sym.Type != nil && !types.IsSend(sym.Type) {
            c.add(diagAt("DTE0020", st.Span,
                "cannot spawn with captured variable '"+varName+"' of non-Send type"))
        }
    }
```

### Captured Variable Analysis

The `collectCapturedVariables` method walks the spawn block AST and finds:
- Identifiers that reference variables from outer scopes
- Excludes locally-defined variables within the spawn block

## Error Messages

| Code | Message |
|------|---------|
| DTE0020 | `cannot spawn with captured variable 'X' of non-Send type 'T'` |

## Adding New Types

When adding a new type to the type system:

1. Add a case in `IsSend()` in `types/send_sync.go`
2. Add a case in `IsSync()` if different from Send
3. Consider if the type holds thread-local resources

## Future Work

- **Send bounds on generics**: `def foo<T: Send>(x: T)`
- **Automatic Send derivation**: Derive Send for structs with all Send fields
- **Sync checking**: Check Sync for shared references (when added)
