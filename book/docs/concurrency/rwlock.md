# RwLock (Reader-Writer Lock)

An **RwLock** protects data and allows either:
- **Multiple readers** at the same time, OR
- **One writer** with exclusive access

This is great when you have data that's read often but rarely written.

## Basic Usage

```desi
import sync

# Create an RwLock protecting a value
let rw = sync.RwLock(42)

# Read access (multiple readers allowed)
using reader = rw.read():
    print(reader.value)  # 42

# Write access (exclusive)
using writer = rw.write():
    print(writer.value)  # Exclusive access
```

## When to Use RwLock

Use RwLock when:
- **Reads are frequent, writes are rare** - Readers don't block each other
- **Data integrity matters** - Writers get exclusive access
- **Performance critical** - Better than Mutex for read-heavy workloads

## Methods

| Method | Returns | What it does |
|--------|---------|-------------|
| `read()` | `ReadGuard<T>` | Get shared read access (blocks if writer active) |
| `write()` | `WriteGuard<T>` | Get exclusive write access (blocks if any access active) |
| `try_read()` | `Option<ReadGuard<T>>` | Try to read without blocking |
| `try_write()` | `Option<WriteGuard<T>>` | Try to write without blocking |

## The using Guard Pattern

**Always use `using` with guards** for automatic unlock:

```desi
# ✅ Correct - lock released automatically
using reader = rw.read():
    print(reader.value)
# Lock released here

# ⚠️ Warning - manual unlock needed
let reader = rw.read()  # DSY0004 warning
print(reader.value)
# Lock NOT released!
```

## Non-Blocking with try Methods

```desi
import sync

let rw = sync.RwLock(42)

match rw.try_read():
    Option.Some(guard): print(guard.value)
    Option.Nothing: print("Couldn't get read lock")
```

## Works with Any Type

```desi
import sync

# Primitives
let rw_int = sync.RwLock(42)
let rw_str = sync.RwLock("hello")
let rw_float = sync.RwLock(3.14)
let rw_bool = sync.RwLock(true)

# Structs
struct Point:
    x: int
    y: int

let rw_point = sync.RwLock(Point(x=10, y=20))

using reader = rw_point.read():
    print(reader.value.x)  # 10
```

## RwLock vs Mutex

| Feature | RwLock | Mutex |
|---------|--------|-------|
| Readers | Multiple concurrent | One at a time |
| Writers | Exclusive | Exclusive |
| Best for | Read-heavy data | General protection |
| Guards | ReadGuard/WriteGuard | MutexGuard |

## Diagnostics

| Code | Severity | Message |
|------|----------|---------|
| `DSY0004` | Warning | ReadGuard without `using` |
| `DSY0005` | Warning | WriteGuard without `using` |

## Common Patterns

### Configuration Data
```desi
# Config is read often, rarely updated
let config = sync.RwLock(load_config())

# Many threads can read simultaneously
using reader = config.read():
    print(reader.value.setting)
```

### Cache with Occasional Updates
```desi
let cache = sync.RwLock(initial_cache())

# Reading (frequent)
using reader = cache.read():
    if reader.value.has_key("data"):
        return reader.value.get("data")

# Writing (rare)
using writer = cache.write():
    # Exclusive access for update
    pass
```
