# Sync Module - Concurrency Primitives in Desi

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [What is the Sync Module?](#what-is-the-sync-module)
3. [Mutex](#mutex)
4. [Usage Guide](#usage-guide)
5. [Type Safety](#type-safety)
6. [Implementation Details](#implementation-details)
7. [Best Practices](#best-practices)
8. [Examples](#examples)

---

## Quick Start

```desi
import sync

def main() -> int:
    let counter = sync.Mutex(42)    # Create mutex protecting int
    let guard = counter.lock()       # Lock and get guard
    print(guard.value)               # Access protected value: 42
    return 0
```

Key features:
- **Generic**: `Mutex<T>` works with any type (int, str, struct, class, list, etc.)
- **Type inference**: `sync.Mutex(42)` infers `Mutex<int>`
- **Safe access**: Guard pattern ensures value is accessed only while locked
- **C runtime backed**: Fast, platform-native mutex implementation

---

## What is the Sync Module?

The `sync` module provides thread-safe concurrency primitives for multi-threaded Desi programs:

| Type | Description |
|------|-------------|
| `Mutex<T>` | Thread-safe wrapper protecting a value of type T |
| `MutexGuard<T>` | RAII guard for safe access to mutex-protected value |
| `Channel<T>` | Bounded message queue for thread communication (planned) |
| `TaskGroup` | Structured concurrency for managing spawned tasks (planned) |

---

## Mutex

### Overview

A `Mutex<T>` (mutual exclusion) protects shared data from concurrent access:

```desi
import sync

# Create mutex wrapping any value
let m = sync.Mutex(100)           # Mutex<int>
let s = sync.Mutex("hello")       # Mutex<str>
let p = sync.Mutex(Point(1, 2))   # Mutex<Point>
```

### Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `lock()` | `MutexGuard<T>` | Acquire lock, block until available |
| `try_lock()` | `Option<MutexGuard<T>>` | Try to acquire lock without blocking |

### MutexGuard

The guard provides safe access to the protected value:

```desi
let guard = mutex.lock()   # Acquire lock
let val = guard.value      # Access the protected value
# Lock released when guard goes out of scope
```

---

## Usage Guide

### Basic Usage

```desi
import sync

def main() -> int:
    # Create mutex with initial value
    let counter = sync.Mutex(0)
    
    # Lock and access
    let guard = counter.lock()
    print(guard.value)  # 0
    
    return 0
```

### With Different Types

```desi
import sync

struct Point:
    x: int
    y: int

def main() -> int:
    # Primitives
    let int_mutex = sync.Mutex(42)
    let str_mutex = sync.Mutex("hello")
    let bool_mutex = sync.Mutex(true)
    
    # Structures
    let point_mutex = sync.Mutex(Point(x=10, y=20))
    let point_guard = point_mutex.lock()
    print(point_guard.value.x)  # 10
    
    # Collections
    let list_mutex = sync.Mutex([1, 2, 3])
    let list_guard = list_mutex.lock()
    print(len(list_guard.value))  # 3
    
    return 0
```

### Try Lock (Non-Blocking)

```desi
import sync

def main() -> int:
    let m = sync.Mutex(42)
    
    match m.try_lock():
        Some(guard):
            print("Acquired lock!")
            print(guard.value)
        Nothing:
            print("Lock busy, try again later")
    
    return 0
```

---

## Type Safety

### Generic Type Inference

The mutex type is inferred from the argument:

```desi
sync.Mutex(42)          # Mutex<int>
sync.Mutex(3.14)        # Mutex<float>
sync.Mutex("text")      # Mutex<str>
sync.Mutex([1, 2, 3])   # Mutex<list<int>>
```

### Guard Type Safety

The guard's `value` field has the same type as the mutex's inner type:

```desi
let m: Mutex<int> = sync.Mutex(42)
let g: MutexGuard<int> = m.lock()
let v: int = g.value   # Type-safe access
```

---

## Implementation Details

### Runtime Structure

The sync module uses a C runtime for platform-native mutex operations:

```c
// Platform-specific (pthreads on Unix, SRWLOCK on Windows)
typedef struct {
    pthread_mutex_t lock;   // Or SRWLOCK on Windows
    void* value;            // Boxed protected value
} DesiMutex;

typedef struct {
    DesiMutex* mutex;       // Reference to parent mutex
    void* value;            // Cached value pointer
} DesiMutexGuard;
```

### Runtime Functions

| C Function | Desi Operation |
|------------|----------------|
| `mutex_new(ptr)` | `sync.Mutex(value)` |
| `mutex_lock(mutex)` | `mutex.lock()` |
| `mutex_try_lock(mutex)` | `mutex.try_lock()` |
| `mutex_unlock(guard)` | Guard destruction |
| `mutex_guard_get(guard)` | `guard.value` |

### Compiler Integration

1. **Type Checking**: `sync.Mutex(value)` resolved in `expr_call.go`
2. **Lowering**: Constructor/methods mapped to C runtime in `lower_call.go`
3. **LLVM Backend**: Runtime function declarations in `emit_call.go`

---

## Best Practices

### ✅ DO

```desi
# DO: Use for shared mutable state
let shared_counter = sync.Mutex(0)

# DO: Keep lock scope minimal
let current = shared_counter.lock().value
# Lock released immediately after value access
```

```desi
# DO: Use for protecting complex data
let shared_config = sync.Mutex(AppConfig(
    debug=true,
    max_connections=100
))
```

### ❌ DON'T

```desi
# DON'T: Hold locks for long operations
let guard = mutex.lock()
expensive_network_call()  # ❌ Blocks other threads!
```

```desi
# DON'T: Use mutex for immutable data
let frozen = sync.Mutex(CONSTANT_VALUE)  # ❌ Unnecessary overhead
```

---

## Examples

### Counter with Mutex

```desi
import sync

def main() -> int:
    let counter = sync.Mutex(0)
    
    # Increment (with spawn for concurrency)
    spawn:
        let g = counter.lock()
        # Note: In-place mutation requires with statement (future)
    
    let final = counter.lock().value
    print(final)
    return 0
```

### Shared Configuration

```desi
import sync

struct Config:
    debug: bool
    max_retries: int

def main() -> int:
    let config = sync.Mutex(Config(debug=true, max_retries=3))
    
    # Read config safely
    let guard = config.lock()
    if guard.value.debug:
        print("Debug mode enabled")
    
    return 0
```

---

## Current Limitations

1. **Import syntax**: Use `import sync` (not `from sync import Mutex`)
2. **With statement**: Not yet enforced for guards (planned for M15)
3. **Value mutation**: Requires explicit copy-back pattern

---

## Implementation Status

### ✅ Implemented

- `sync.Mutex(value)` constructor
- `mutex.lock()` blocking acquire
- `mutex.try_lock()` non-blocking acquire
- `guard.value` protected access
- Generic type support (Mutex<T>)

### 🚧 Planned

- [ ] `with mutex.lock() as guard:` syntax enforcement
- [ ] `Channel<T>` for message passing
- [ ] `TaskGroup` for structured concurrency
- [ ] `RwLock<T>` for reader-writer locks

---

**This document is the reference for the sync module in Desi.** For runtime implementation, see `compiler/runtime/mutex.c`. For type system integration, see `compiler/internal/check/expr_field.go`.
