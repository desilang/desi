# Mutex - Thread-Safe Shared State

A **Mutex** (mutual exclusion) protects shared data from concurrent access. When multiple tasks need to read or write the same value, a Mutex ensures only one task can access it at a time.

## Creating a Mutex

Use `mutex_new(value)` to create a Mutex protecting any value:

```desi
# Protect an integer counter
let counter = mutex_new(0)

# Protect a string
let name = mutex_new("unknown")

# Protect a custom struct
struct Config:
    host: str
    port: int

let config = mutex_new(Config(host="localhost", port=8080))
```

The type is inferred automatically: `mutex_new(42)` creates `Mutex[int]`.

## Locking and Accessing Values

To access the protected value, you must **lock** the mutex first:

```desi
let counter = mutex_new(0)

# Lock the mutex to get a guard
let guard = counter.lock()

# Access the value through the guard
let current = guard.value
print(current)  # 0
```

### The Guard Pattern

The `.lock()` method returns a `MutexGuard[T]`, not the value directly. This ensures:
1. The lock is held while you access the value
2. You can't accidentally access the value without locking

```desi
let counter = mutex_new(100)
let guard = counter.lock()

# guard.value is the protected int
print(guard.value)  # 100

# When guard goes out of scope, the lock is released
```

## Non-Blocking Lock with `try_lock()`

If you don't want to wait for the lock:

```desi
let m = mutex_new(42)

match m.try_lock():
    Option.Some(guard):
        print("Got lock: " + str(guard.value))
    Option.Nothing:
        print("Lock is held by another task")
```

## Complete Examples

### Protecting a Counter

```desi
struct Counter:
    pub mut count: int

def main() -> int:
    let counter = mutex_new(0)
    
    # Lock to read
    let guard = counter.lock()
    print(guard.value)  # 0
    
    return 0
```

### Protecting a Configuration

```desi
struct ServerConfig:
    host: str
    port: int
    max_connections: int

def main() -> int:
    let config = mutex_new(ServerConfig(
        host="0.0.0.0",
        port=8080,
        max_connections=100
    ))
    
    let guard = config.lock()
    print("Server on port: " + str(guard.value.port))
    
    return 0
```

### Protecting a Class Instance

```desi
class Logger:
    pub mut entries: list[str]
    
    pub def __new__(self):
        self.entries = []
    
    pub def count(self) -> int:
        return len(self.entries)

def main() -> int:
    let logger = Logger()
    let shared_logger = mutex_new(logger)
    
    let guard = shared_logger.lock()
    print("Log entries: " + str(guard.value.count()))
    
    return 0
```

## Best Practices

### 1. Keep Critical Sections Short

Hold the lock for the minimum time necessary:

```desi
# Good: Short critical section
let guard = counter.lock()
let value = guard.value
# guard is released

# Process value without holding lock
let result = expensive_computation(value)
```

### 2. Use `try_lock()` to Avoid Deadlocks

When you can't afford to wait:

```desi
match resource.try_lock():
    Option.Some(guard):
        use_resource(guard.value)
    Option.Nothing:
        # Do something else or retry later
        fallback_behavior()
```

### 3. Prefer Constructor Initialization

Initialize values in `__new__` rather than assigning after construction:

```desi
# Good: Use __new__
let c = Counter(100)
let m = mutex_new(c)

# Avoid: Assigning after construction
let c = Counter()
c.count := 100  # Requires :=
let m = mutex_new(c)
```

## Supported Types

Mutex works with all Desi types:

| Type | Example |
|------|---------|
| `int` | `mutex_new(42)` |
| `float` | `mutex_new(3.14)` |
| `bool` | `mutex_new(true)` |
| `str` | `mutex_new("hello")` |
| `struct` | `mutex_new(Point(x=1, y=2))` |
| `class` | `mutex_new(Counter(0))` |
| `list` | `mutex_new([1, 2, 3])` |

## When to Use Mutex

Use Mutex when:
- Multiple tasks need to read/write the same data
- You need to protect invariants across multiple fields
- Order of access matters (first-come, first-served)

Consider alternatives:
- **Channel**: When passing data between tasks (message passing)
- **Atomic**: For simple counters (when available)

## See Also

- [Channels](./channels.md) - Message passing between tasks
- [Concurrency Overview](./concurrency.md) - Async/await and tasks
