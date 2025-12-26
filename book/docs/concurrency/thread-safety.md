# Thread Safety in Desi

Desi provides compile-time guarantees to help you write safe concurrent code. This page explains the **Send** and **Sync** traits that determine which types can be used safely across task boundaries.

## The Send Trait

A type is **Send** if it's safe to transfer ownership to another task. Most types in Desi are Send:

| Type | Send? | Why |
|------|-------|-----|
| `int`, `float`, `bool`, `str` | ✓ | Primitives are always safe |
| `list[T]`, `set[T]`, `dict[K,V]` | ✓ | If elements are Send |
| `Channel[T]` | ✓ | Designed for cross-task use |
| `Mutex[T]` | ✓ | Thread-safe by design |
| **`MutexGuard[T]`** | ✗ | Holds a lock - see below |

## MutexGuard is Not Send

The one important exception is `MutexGuard`. When you lock a mutex, the guard **cannot** be sent to another task:

```desi
def main() -> int:
    let counter = mutex_new(42)
    let guard = counter.lock()  # guard holds the lock
    
    spawn:
        # ERROR: cannot spawn with captured MutexGuard
        print(guard.value)
    
    return 0
```

This protects you from:
- **Lock ownership confusion**: The lock belongs to the task that acquired it
- **Unlock issues**: The guard must be released in the same task

### The Fix

Access the mutex inside the spawn block instead:

```desi
def main() -> int:
    let counter = mutex_new(42)
    
    spawn:
        # Correct: Lock inside the spawned task
        let guard = counter.lock()  # Mutex is Send, so this works
        print(guard.value)
    
    return 0
```

## The Sync Trait

A type is **Sync** if it's safe to share references across tasks. This matters when multiple tasks need to read the same data simultaneously.

Most immutable data is Sync. Mutex is Sync because it provides safe interior mutability.

## Common Patterns

### Sharing State with Mutex

```desi
# Mutex is Send - can be shared across tasks
let shared_counter = mutex_new(0)

spawn:
    let guard = shared_counter.lock()
    # Access guard.value here
```

### Passing Data via Channels

```desi
# Channels are Send - perfect for task communication
let ch = channel_new(10)
let sender = ch.sender()

spawn:
    sender.send(42)  # Send data to another task
```

## Summary

| Concept | Meaning |
|---------|---------|
| **Send** | Can be transferred to another task |
| **Sync** | Can be shared by reference across tasks |
| **MutexGuard** | NOT Send - lock must stay in original task |

The compiler checks these constraints automatically. If you try to capture a non-Send type in a `spawn` block, you'll get a compile-time error.
