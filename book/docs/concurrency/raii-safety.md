# RAII and Resource Safety

Desi uses **RAII** (Resource Acquisition Is Initialization) to ensure resources like locks, files, and channels are properly cleaned up. This guide covers best practices for safe resource management.

## The `using` Statement

The `using` statement creates RAII guards that automatically release resources:

```desi
import sync

def process_data():
    let mutex = sync.Mutex(0)
    
    using guard = mutex.lock():
        guard.value = guard.value + 1
        # Lock automatically released at end of block
    
    # Safe to use mutex again - lock was released
```

## Why Guards Can't Escape

Guards (like `MutexGuard`, `ReadGuard`, `WriteGuard`) are tied to a scope. If they escaped, the resource would remain locked forever:

```desi
# ❌ COMPILE ERROR: DSY0002
def bad_function() -> MutexGuard<int>:
    let m = sync.Mutex(42)
    return m.lock()  # Error: cannot return guard type
```

Instead, extract the value you need:

```desi
# ✓ Correct: Return the value, not the guard
def good_function() -> int:
    let m = sync.Mutex(42)
    using guard = m.lock():
        return guard.value  # Returns 42, guard cleaned up
```

## Conditionals Inside Using Blocks

You can use conditionals inside `using` blocks:

```desi
import sync

def conditional_processing() -> int:
    let m = sync.Mutex(50)
    
    using guard = m.lock():
        if guard.value > 40:
            return guard.value + 10
        else:
            return guard.value - 10
    
    return 0  # Never reached (both branches return)
```

The lock is properly released on any exit path.

## Best Practices

### 1. Keep Critical Sections Short

```desi
# ✓ Good: Short lock duration
using guard = data.lock():
    let value = guard.value  # Copy what you need
# Process outside the lock
expensive_computation(value)
```

### 2. Don't Store Guards

```desi
# ❌ Bad: Storing a guard extends lock lifetime
let saved_guard = mutex.lock()  # Holds lock until function ends

# ✓ Good: Use `using` for automatic cleanup
using guard = mutex.lock():
    use_value(guard.value)
```

### 3. Nested Using Blocks

```desi
using tx = chan.sender():
    using rx = chan.receiver():
        tx.send(42)
        print(rx.recv())
    # rx cleaned up here
# tx cleaned up here
```

## Resource Types and RAII

| Type | Cleanup Action |
|------|---------------|
| `MutexGuard<T>` | Releases mutex lock |
| `ReadGuard<T>` | Releases read lock |
| `WriteGuard<T>` | Releases write lock |
| `Sender<T>` | Closes sender channel end |
| `Receiver<T>` | Closes receiver channel end |
| Arena | Frees all arena allocations |

## Common Errors

### DSY0002: Cannot Return Guard

You tried to return a guard from a function:

```desi
def get_guard() -> MutexGuard<int>:
    return mutex.lock()  # ❌ DSY0002
```

**Fix**: Return the value instead, or restructure to keep the guard in scope:

```desi
def get_value() -> int:
    using guard = mutex.lock():
        return guard.value  # ✓
```

## See Also

- [Mutex](./mutex.md) - Thread-safe shared state
- [RwLock](./rwlock.md) - Reader-writer locks
- [Channels](./channels.md) - Message passing
