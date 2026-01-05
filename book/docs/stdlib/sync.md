# sync - Concurrency Primitives

The `sync` module provides thread-safe concurrency primitives for multi-threaded programs.

## Import

```desi
import sync
```

## Types

### Mutex&lt;T&gt;

Thread-safe wrapper protecting a value of any type.

```desi
let m = sync.Mutex(42)     # Mutex<int>
let s = sync.Mutex("hi")   # Mutex<str>
```

**Methods:**

| Method | Returns | Description |
|--------|---------|-------------|
| `lock()` | `MutexGuard<T>` | Acquire lock (blocking) |
| `try_lock()` | `Option<MutexGuard<T>>` | Try lock (non-blocking) |

### MutexGuard&lt;T&gt;

RAII guard for safe access to mutex-protected values.

```desi
let guard = mutex.lock()
print(guard.value)  # Access the protected value
```

## Examples

### Basic Mutex

```desi
import sync

let counter = sync.Mutex(0)
let guard = counter.lock()
print(guard.value)  # 0
```

### With Structs

```desi
import sync

struct Point:
    x: int
    y: int

let p = sync.Mutex(Point(x=10, y=20))
let guard = p.lock()
print(guard.value.x)  # 10
```

## See Also

- [Mutex Tutorial](../concurrency/mutex.md) - Detailed guide
- [Channels](../concurrency/channels.md) - Message passing
- [TaskGroup](../concurrency/taskgroup.md) - Structured concurrency
