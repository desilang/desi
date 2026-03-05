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
| `RwLock<T>` | Reader-writer lock: multiple readers OR one writer |
| `ReadGuard<T>` | RAII guard for shared read access |
| `WriteGuard<T>` | RAII guard for exclusive write access |
| `Semaphore` | Counting semaphore for limiting concurrent access |
| `Atomic` | Lock-free atomic integer operations |
| `Channel<T>` | Bounded message queue for thread communication |
| `Sender<T>` | Send handle for Channel |
| `Receiver<T>` | Receive handle for Channel |
| `TaskGroup` | Structured concurrency for managing spawned tasks |
| `Supervisor` | Thread pool with work queue and auto-restart |

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

## RwLock

### Overview

An `RwLock<T>` (reader-writer lock) allows multiple readers OR one exclusive writer:

```desi
import sync

let rw = sync.RwLock(42)         # RwLock<int>

# Multiple readers allowed concurrently
using reader = rw.read():
    print(reader.value)          # 42

# Only one writer at a time (exclusive)
using writer = rw.write():
    print(writer.value)          # Exclusive access
```

### RwLock Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `read()` | `ReadGuard<T>` | Acquire shared read lock (blocks) |
| `write()` | `WriteGuard<T>` | Acquire exclusive write lock (blocks) |
| `try_read()` | `Option<ReadGuard<T>>` | Try to acquire read lock without blocking |
| `try_write()` | `Option<WriteGuard<T>>` | Try to acquire write lock without blocking |

### ReadGuard and WriteGuard

Guards provide safe access to the protected value:

```desi
# Read guard - shared access (read-only)
using rg = rw.read():
    let val = rg.value   # Read the value

# Write guard - exclusive access (read-write)
using wg = rw.write():
    let val = wg.value   # Access exclusively
```

### Diagnostics

| Code | Severity | Condition | Message |
|------|----------|-----------|---------|
| `DSY0004` | Warning | ReadGuard without `using` | "Consider using 'using guard = rw.read():'" |
| `DSY0005` | Warning | WriteGuard without `using` | "Consider using 'using guard = rw.write():'" |

---

## Semaphore

### Overview

A `Semaphore` is a counting semaphore that limits concurrent access:

```desi
import sync

let sem = sync.Semaphore(3)  # Allow 3 concurrent accesses

sem.acquire()    # Get a permit (blocks if none available)
# ... do work ...
sem.release()    # Return the permit
```

### Semaphore Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `acquire()` | `none` | Acquire a permit (blocks if none available) |
| `release()` | `none` | Release a permit (wakes one waiting thread) |
| `try_acquire()` | `bool` | Try to acquire without blocking (returns true if acquired) |

### Example: Rate Limiting

```desi
import sync

# Allow max 2 concurrent operations
let sem = sync.Semaphore(2)

sem.acquire()
print("Worker 1 started")
sem.release()

sem.acquire()
print("Worker 2 started")
sem.release()
```

---

## Atomic

### Overview

An `Atomic` provides lock-free atomic integer operations using C11 atomics:

```desi
import sync

let counter = sync.Atomic(0)  # Create atomic integer

let val = counter.load()      # Read atomically
counter.store(100)            # Write atomically
let new = counter.inc()       # Increment, returns new value
```

### Atomic Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `load()` | `int` | Read the current value atomically |
| `store(value)` | `none` | Write a new value atomically |
| `add(delta)` | `int` | Add delta and return the NEW value |
| `sub(delta)` | `int` | Subtract delta and return the NEW value |
| `inc()` | `int` | Increment by 1 and return the NEW value |
| `dec()` | `int` | Decrement by 1 and return the NEW value |
| `compare_exchange(expected, desired)` | `bool` | CAS: if current == expected, set to desired |
| `exchange(new_value)` | `int` | Replace value and return the OLD value |

### Example: Lock-Free Counter

```desi
import sync

let counter = sync.Atomic(0)

let v1 = counter.inc()  # 1
let v2 = counter.inc()  # 2
let v3 = counter.add(10)  # 12
```

---

## Channel

### Overview

A `Channel<T>` provides thread-safe message passing between concurrent tasks:

```desi
import sync

let ch = sync.Channel(10)   # Bounded channel with capacity 10
let tx = ch.sender()        # Get sender handle
let rx = ch.receiver()      # Get receiver handle

tx.send(42)                 # Send value
match rx.recv():            # Receive value
    Option.Some(v): print(v)
    Option.Nothing: print("Channel closed")
```

### Channel Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `sender()` | `Sender<T>` | Get a sender handle |
| `receiver()` | `Receiver<T>` | Get a receiver handle |
| `close()` | `none` | Close the channel |

### Sender Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `send(value)` | `bool` | Send value (blocks if full) |
| `try_send(value)` | `bool` | Try to send without blocking |

### Receiver Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `recv()` | `Option<T>` | Receive value (blocks if empty) |
| `try_recv()` | `Option<T>` | Try to receive without blocking |

### RAII Best Practices for Channels

While Sender and Receiver can be used without `using`, it is **recommended** for proper cleanup:

```desi
import sync

def main() -> int:
    let ch = sync.Channel(10)
    
    # ✅ RECOMMENDED: using guard for sender/receiver
    using tx = ch.sender():
        tx.send(42)
    # tx automatically dropped here
    
    ch.close()
    return 0
```

> [!TIP]
> Using `using tx = ch.sender():` ensures the sender handle is properly dropped, which updates reference counts.

### Diagnostics

| Code | Severity | Condition | Message |
|------|----------|-----------|---------|
| `DSY0002` | Warning | Sender without `using` | "Consider using 'using tx = ch.sender():'" |
| `DSY0003` | Warning | Receiver without `using` | "Consider using 'using rx = ch.receiver():'" |

---

## TaskGroup

### Overview

A `TaskGroup` provides structured concurrency for managing spawned tasks:

```desi
import sync

def worker() -> none:
    print("Working...")

def main() -> int:
    using tg = sync.TaskGroup():    # REQUIRED: using guard (DSY0001 error if not)
        tg.run(worker)              # Spawn worker concurrently
        tg.run(worker)              # Spawn another
        tg.wait()                   # Wait for all tasks to complete
    return 0
```

> [!IMPORTANT]
> **TaskGroup MUST be used with `using` guard.** Creating a TaskGroup without `using` is a compile error (DSY0001).
> This ensures proper RAII cleanup and prevents resource leaks.

### TaskGroup Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `run(fn)` | `none` | Spawn a function in the task group |
| `wait()` | `none` | Wait for all tasks to complete |
| `cancel()` | `none` | Cancel all tasks in the group |
| `is_cancelled()` | `bool` | Check if group is cancelled |

### Function Requirements for `run()`

Functions passed to `tg.run()` must:
- Return **`none`**
- Can be named functions, lambdas, or **bound methods** (instance methods)

```desi
# Valid worker function
def my_worker() -> none:
    print("I'm running concurrently!")

# Usage:
using tg = sync.TaskGroup():
    tg.run(my_worker)
    tg.wait()
```

### Bound Methods (Class Instance Methods)

You can pass instance methods directly to `run()`:

```desi
class Worker:
    pub mut id: int
    
    pub def process(self) -> none:
        print("Worker", self.id, "processing...")

def main() -> int:
    let mut w1 = Worker()
    let mut w2 = Worker()
    w1.id := 1
    w2.id := 2
    
    using tg = sync.TaskGroup():
        tg.run(w1.process)  # Bound method - captures w1
        tg.run(w2.process)  # Bound method - captures w2
        tg.wait()
    
    return 0
```

**How bound methods work:**
- `w1.process` creates a **bound method** - a callable that captures both the method and receiver
- When executed, `self` is automatically set to `w1`
- The receiver must outlive the task (TaskGroup's `wait()` ensures this)

### Closure Capture in `run()`

Lambdas can capture outer variables using **value-copy semantics**:

```desi
let msg = "Hello from closure!"
let count = 42

using tg = sync.TaskGroup():
    # Lambda captures `msg` and `count` by value
    tg.run(lambda: print(msg, count))
    tg.wait()
```

**Key behaviors:**
- Captured values are **copied** when `run()` is called
- Changes to the original variable don't affect the captured copy
- Safe for concurrent access (no data races)

```desi
# Multiple captures example
let items = ["a", "b", "c"]
using tg = sync.TaskGroup():
    for i in range(len(items)):
        let item = items[i]
        tg.run(lambda: print("Processing:", item))
    tg.wait()
```

> [!IMPORTANT]
> All captured values are copied at `run()` time. For shared mutable state, use explicit synchronization primitives like `Mutex` or `Atomic`.

### Diagnostics

| Code | Severity | Condition | Message |
|------|----------|-----------|---------|
| `DSY0001` | **Error** | TaskGroup without `using` | "TaskGroup must be used with 'using' guard" |

---

## Supervisor

### Overview

A `Supervisor` provides a thread pool with work queue and persistent children with auto-restart. Inspired by Elixir/OTP supervision trees.

```desi
import sync

def worker():
    print("Worker task executed")

def main() -> int:
    using sup = sync.Supervisor():   # REQUIRED: using guard (DSY0001 error if not)
        sup.submit(worker)           # Enqueue work to pool
        sup.submit(worker)
        sup.stop()                   # Drain + join + free
    return 0
```

> [!IMPORTANT]
> **Supervisor MUST be used with `using` guard.** Creating a Supervisor without `using` is a compile error (DSY0001).
> This ensures proper RAII cleanup: `supervisor_stop()` is called automatically.

### Supervisor Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `submit(fn)` | `none` | Enqueue a task to the worker pool |
| `start_child(fn)` | `none` | Start a persistent child (auto-restarted on crash) |
| `stop()` | `none` | Drain queue, signal shutdown, join all threads |
| `pool_size()` | `int` | Get number of pool worker threads |
| `child_count()` | `int` | Get number of persistent children |
| `is_running()` | `bool` | Check if supervisor is still running |

### Restart Strategies

| Strategy | Behavior |
|----------|----------|
| `ONE_FOR_ONE` (default) | Restart only the crashed child |
| `ONE_FOR_ALL` | Restart all children if one crashes |

### Pool vs Persistent Children

**Pool workers** (`submit`): Short-lived tasks dispatched from a ring buffer queue. Pool threads dequeue and execute tasks, then wait for more work.

**Persistent children** (`start_child`): Long-running tasks that are auto-restarted if they exit. Rate-limited: max 5 restarts per 60-second window (configurable).

### Idempotent Stop

`supervisor_stop()` is idempotent — safe to call multiple times. This is critical for RAII safety because both explicit `sup.stop()` and the automatic `__close__` call `supervisor_stop()`.

### Implementation Details

#### C Runtime (`supervisor.c`)

| C Function | Desi Operation |
|------------|----------------|
| `supervisor_new(strategy, workers)` | `sync.Supervisor()` |
| `supervisor_submit(sup, fn, arg)` | `sup.submit(fn)` |
| `supervisor_start_child(sup, fn, arg)` | `sup.start_child(fn)` |
| `supervisor_stop(sup)` | `sup.stop()` / RAII `__close__` |
| `supervisor_pool_size(sup)` | `sup.pool_size()` |
| `supervisor_child_count(sup)` | `sup.child_count()` |
| `__supervisor_is_running(sup)` | `sup.is_running()` |

#### Performance

- Ring buffer work queue: O(1) enqueue/dequeue, dynamic growth
- Lock contention minimized: workers only hold lock during dequeue
- No allocation per task submission (pre-allocated ring buffer)

#### Compiler Integration

1. **Type**: `types/supervisor.go` — `*types.Supervisor` type
2. **Type Checking**: `expr_call.go` — constructor + method dispatch (intercepted before class dispatch)
3. **Field Resolution**: `expr_field.go` — method signatures via `resolveSupervisorMethod`
4. **Lowering**: `lower_call.go` — constructor → `supervisor_new(0, 4)`, methods → C runtime calls

### Diagnostics

| Code | Severity | Condition | Message |
|------|----------|-----------|---------|
| `DSY0001` | **Error** | Supervisor without `using` | "Supervisor must be used with 'using' guard" |

---

## Global Shared State

Sync primitives can be declared at **module scope** for shared state across functions. This is essential for server applications that need to track state across requests.

### Global Atomic Counter

```desi
import sync
import http
from http import Request

let request_count = sync.Atomic(0)   # Module-scope — no UPPER_CASE warning

def handle_request(req: Request) -> Any:
    request_count.inc()               # Thread-safe increment
    let count = request_count.load()
    return http.html(200, raw="<h1>Request #" + str(count) + "</h1>")

def main() -> int:
    let srv = http.server(9090)
    http.serve(srv, handle_request)
    return 0
```

### Global Mutex for Complex State

```desi
import sync

let config = sync.Mutex({"debug": true, "max_retries": 3})

def get_config() -> Any:
    let guard = config.lock()
    return guard.value
```

### Supported Global Sync Types

All sync constructors work at module scope:

| Declaration | Type |
|-------------|------|
| `let counter = sync.Atomic(0)` | `Atomic` |
| `let data = sync.Mutex(value)` | `Mutex<T>` |
| `let rw = sync.RWLock(value)` | `RwLock<T>` |
| `let ch = sync.Channel(10)` | `Channel<T>` |
| `let sem = sync.Semaphore(3)` | `Semaphore` |

> [!NOTE]
> Sync globals are exempt from the UPPER_CASE naming convention because they hold **mutable shared state** by design. The compiler detects `sync.*` constructors and suppresses the naming warning.

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

## Import Syntax

Both import styles are fully supported:

```desi
# Full module import
import sync
let m = sync.Mutex(42)

# Direct import
from sync import Mutex
let m = Mutex(42)
```

---

## Current Limitations

1. **With statement**: Not yet enforced for guards (planned for M15)
2. **Value mutation**: Requires explicit copy-back pattern

---

## Implementation Status

### ✅ Implemented

- `sync.Mutex(value)` constructor
- `mutex.lock()` blocking acquire
- `mutex.try_lock()` non-blocking acquire
- `guard.value` protected access
- `using guard = mutex.lock():` RAII pattern
- Generic type support (Mutex<T>)
- `sync.Channel(capacity)` constructor
- `ch.sender()` / `ch.receiver()` handles
- `tx.send(v)` / `tx.try_send(v)` for sending
- `rx.recv()` / `rx.try_recv()` for receiving
- `sync.TaskGroup()` constructor
- `tg.run(fn)` spawn tasks
- `tg.wait()` wait for all tasks
- `tg.cancel()` cancel all tasks
- `tg.is_cancelled()` check cancel status
- `sync.Supervisor()` constructor
- `sup.submit(fn)` enqueue work to pool
- `sup.start_child(fn)` persistent children with auto-restart
- `sup.stop()` drain + join + free (idempotent)
- `sup.pool_size()`, `sup.child_count()`, `sup.is_running()`
- ONE_FOR_ONE restart strategy
- Both `import sync` and `from sync import` styles
- **Global shared state**: all sync types work at module scope
- **Warning suppression**: sync globals exempt from UPPER_CASE naming

### 🚧 Planned

- [ ] ONE_FOR_ALL full implementation (restart all children)
- [ ] Configurable restart limits per Supervisor instance
- [ ] Supervisor trees (nested supervisors)

---

**This document is the reference for the sync module in Desi.** For runtime implementation, see `compiler/runtime/mutex.c`. For type system integration, see `compiler/internal/check/expr_field.go`.
